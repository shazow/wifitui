{
  self,
  system,
  pkgs,
}:

let
  lib = pkgs.lib;

  # Python helpers to drive the TUI in tmux. randomize_mac_from_tui opens the
  # active network, checks "Randomize MAC address" and presses Connect.
  tuiHelpers = ''
    def tui_screen():
        return machine.succeed("tmux capture-pane -p -t tui")

    def tui_keys(*keys):
        machine.succeed("tmux send-keys -t tui " + " ".join(keys))
        machine.sleep(1)

    def wait_for_tui_text(text):
        try:
            machine.wait_until_succeeds(f"tmux capture-pane -p -t tui | grep -qF '{text}'", timeout=60)
        except Exception:
            print(machine.execute("tmux capture-pane -p -t tui")[1])
            raise

    def randomize_mac_from_tui(ssid):
        # Redirect tmux's stdio so the test driver doesn't wait on the tmux
        # server, which keeps running in the background.
        machine.succeed("tmux new-session -d -s tui -x 120 -y 50 'NO_COLOR=1 wifitui tui' </dev/null >/dev/null 2>&1")
        wait_for_tui_text(ssid)
        tui_keys("Enter")  # The active network is sorted first.
        wait_for_tui_text("Randomize MAC address")
        tui_keys("BTab")  # From the buttons to the checkbox.
        tui_keys("Space")
        screen = tui_screen()
        assert "[x] Randomize MAC address" in screen, screen
        tui_keys("Tab")  # Back to the buttons, where Connect is selected.
        tui_keys("Enter")

    def wait_for_new_mac(old_mac, connected_cmd, diagnostics):
        try:
            machine.wait_until_succeeds(
                f"test \"$(cat /sys/class/net/wlan1/address)\" != {old_mac} && {connected_cmd}",
                timeout=60,
            )
        except Exception:
            print(diagnostics())
            raise
        finally:
            machine.execute("tmux kill-session -t tui")
  '';

  # wifiBackend is NetworkManager's wifi.backend: "wpa_supplicant" or "iwd".
  mkNetworkManagerTest =
    { name, wifiBackend }:
    let
      viaIwd = wifiBackend == "iwd";
    in
    pkgs.nixosTest {
    inherit name;

    nodes.machine =
      { lib, pkgs, ... }:
      {
        config = lib.mkMerge [
      {
        virtualisation.memorySize = 1024;

        boot.kernelModules = [ "mac80211_hwsim" ];
        boot.extraModprobeConfig = ''
          options mac80211_hwsim radios=2
        '';

        environment.systemPackages = [
          self.packages.${system}.default
          pkgs.networkmanager
          pkgs.tmux
        ];

        networking.firewall.enable = false;

        networking.networkmanager = {
          enable = true;
          wifi.backend = wifiBackend;
          unmanaged = [
            "interface-name:wlan0"
            "interface-name:wlan0_*"
          ];
        };

        services.hostapd = {
          enable = true;
          radios.wlan0 = {
            band = "2g";
            channel = 6;
            networks = {
              wlan0 = {
                ssid = "Airport_Free_WiFi";
                bssid = "02:00:00:00:00:00";
                authentication.mode = "none";
              };
              wlan0_0 = {
                ssid = "Home_Network";
                bssid = "06:00:00:00:00:00";
                authentication = {
                  mode = "wpa2-sha1";
                  wpaPassword = "supersecret";
                };
              };
              wlan0_1 = {
                ssid = "Test_IoT_Device";
                bssid = "0a:00:00:00:00:00";
                authentication.mode = "none";
              };
              wlan0_2 = {
                ssid = "Secret_Corp_Net";
                bssid = "0e:00:00:00:00:00";
                authentication.mode = "none";
                settings.ignore_broadcast_ssid = lib.mkForce 1;
              };
            };
          };
        };
        systemd.services.hostapd = {
          after = [ "sys-subsystem-net-devices-wlan0.device" ];
          bindsTo = [ "sys-subsystem-net-devices-wlan0.device" ];
        };

        systemd.services.hwsim-ap-network = {
          after = [ "hostapd.service" ];
          requires = [ "hostapd.service" ];
          wantedBy = [ "multi-user.target" ];
          path = [ pkgs.iproute2 ];
          serviceConfig.Type = "oneshot";
          script = ''
            for _ in $(seq 1 50); do
              if [ -d /sys/class/net/wlan0_0 ]; then
                break
              fi
              sleep 0.1
            done
            ip address replace 192.168.76.1/24 dev wlan0_0
            ip link set wlan0_0 up
          '';
        };

        services.dnsmasq = {
          enable = true;
          resolveLocalQueries = false;
          settings = {
            interface = "wlan0_0";
            bind-interfaces = true;
            dhcp-authoritative = true;
            dhcp-range = [ "192.168.76.50,192.168.76.100,255.255.255.0,1h" ];
            dhcp-option = [
              "3,192.168.76.1"
              "6,192.168.76.1"
            ];
          };
        };
        systemd.services.dnsmasq = {
          after = [ "hwsim-ap-network.service" ];
          requires = [ "hwsim-ap-network.service" ];
        };
      }
      (lib.mkIf viaIwd {
        # iwd only honors per-network MAC randomization with this.
        networking.wireless.iwd.settings.General.AddressRandomization = "network";
        # hostapd runs the access point on wlan0, so iwd must leave it alone.
        systemd.services.iwd.serviceConfig.ExecStart = [
          ""
          "${pkgs.iwd}/libexec/iwd --nointerfaces wlan0"
        ];
      })
        ];
      };

    testScript = ''
      import json

      ${tuiHelpers}

      def list_networks(scan=False, all_networks=False):
          flags = ["--json"]
          if all_networks:
              flags.append("--all")
          if scan:
              flags.append("--scan")
          return json.loads(machine.succeed("NO_COLOR=1 wifitui list " + " ".join(flags))) or []

      def network_by_ssid(networks, ssid):
          for network in networks:
              if network["SSID"] == ssid:
                  return network
          raise AssertionError(f"missing {ssid}: {networks}")

      def wait_for_visible_ssids(required, forbidden=()):
          required = set(required)
          forbidden = set(forbidden)
          networks = []
          for _ in range(5):
              networks = list_networks(scan=True)
              ssids = {network["SSID"] for network in networks}
              if required.issubset(ssids) and forbidden.isdisjoint(ssids):
                  return networks
              machine.sleep(2)
          raise AssertionError(f"wanted {required} without {forbidden}: {networks}")

      start_all()

      machine.wait_for_unit("NetworkManager.service")
      machine.wait_until_succeeds("test -d /sys/class/net/wlan0")
      machine.wait_until_succeeds("test -d /sys/class/net/wlan1")
      machine.wait_for_unit("hostapd.service")
      machine.succeed("rfkill unblock all")
      machine.succeed("nmcli radio wifi on")
      machine.wait_until_succeeds("nmcli -t -f DEVICE,TYPE device | grep '^wlan1:wifi$'")
      machine.wait_until_succeeds("nmcli -t -f DEVICE,STATE device | grep -E '^wlan1:(disconnected|connected)$'")

      networks = wait_for_visible_ssids(
          ["Airport_Free_WiFi", "Home_Network", "Test_IoT_Device"],
          ["Secret_Corp_Net"],
      )

      details = machine.succeed("NO_COLOR=1 wifitui show Home_Network")
      assert "SSID: Home_Network" in details, details
      assert "Secure: true" in details, details
      assert "Visible: true" in details, details

      machine.succeed("NO_COLOR=1 wifitui connect --passphrase supersecret --security wpa --retry-for 20s:2s Home_Network")
      machine.wait_until_succeeds("nmcli -t -f ACTIVE,SSID dev wifi | grep '^yes:Home_Network$'")

      connected = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert connected["IsActive"], connected
      assert connected["IsKnown"], connected

      saved_details = machine.succeed("NO_COLOR=1 wifitui show Home_Network")
      assert "Passphrase: supersecret" in saved_details, saved_details
      assert "Active: true" in saved_details, saved_details
      assert "Known: true" in saved_details, saved_details

      rescanned = network_by_ssid(list_networks(scan=True, all_networks=True), "Home_Network")
      assert rescanned["IsActive"], rescanned
      assert rescanned["IsKnown"], rescanned

      # Enable MAC randomization for the active network from the TUI, like a
      # user would, and check that it reconnects with a new MAC address.
      via_iwd = ${if viaIwd then "True" else "False"}

      def mac_diagnostics():
          parts = [
              "TUI screen:\n" + machine.execute("tmux capture-pane -p -t tui")[1],
              "cloned-mac-address: " + machine.execute("nmcli -g 802-11-wireless.cloned-mac-address connection show Home_Network")[1],
              machine.execute("ip link show wlan1")[1],
              machine.execute("journalctl -u NetworkManager --no-pager | grep -iE 'hw-addr|hwaddr|mac|iwd' | tail -n 40")[1],
          ]
          if via_iwd:
              parts.append(machine.execute("cat /etc/iwd/main.conf /var/lib/iwd/*.psk")[1])
              parts.append(machine.execute("journalctl -u iwd --no-pager | tail -n 40")[1])
          return "\n".join(parts)

      initial_mac = machine.succeed("cat /sys/class/net/wlan1/address").strip()
      randomize_mac_from_tui("Home_Network")
      wait_for_new_mac(
          initial_mac,
          "nmcli -t -f ACTIVE,SSID dev wifi | grep -q '^yes:Home_Network$'",
          mac_diagnostics,
      )
      cloned = machine.succeed("nmcli -g 802-11-wireless.cloned-mac-address connection show Home_Network").strip()
      assert cloned == "random", cloned
      if via_iwd:
          # NetworkManager copies the setting into iwd's settings file.
          machine.succeed("grep -qx AlwaysRandomizeAddress=true /var/lib/iwd/Home_Network.psk")

      randomized = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert randomized["RandomizeMAC"], randomized

      machine.succeed("nmcli connection modify Home_Network connection.autoconnect no")
      machine.succeed("nmcli device disconnect wlan1")
      machine.wait_until_succeeds("nmcli -t -f DEVICE,STATE device | grep '^wlan1:disconnected$'")
      disconnected = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert not disconnected["IsActive"], disconnected
      assert disconnected["IsKnown"], disconnected

      machine.succeed("nmcli connection delete id Home_Network")
      forgotten = network_by_ssid(list_networks(scan=True, all_networks=True), "Home_Network")
      assert not forgotten["IsActive"], forgotten
      assert not forgotten["IsKnown"], forgotten
    '';
  };
in
lib.optionalAttrs pkgs.stdenv.isLinux {
  networkmanager-hwsim = mkNetworkManagerTest {
    name = "wifitui-networkmanager-hwsim";
    wifiBackend = "wpa_supplicant";
  };

  networkmanager-iwd-hwsim = mkNetworkManagerTest {
    name = "wifitui-networkmanager-iwd-hwsim";
    wifiBackend = "iwd";
  };

  iwd-hwsim = pkgs.nixosTest {
    name = "wifitui-iwd-hwsim";

    nodes.machine =
      { lib, pkgs, ... }:
      {
        virtualisation.memorySize = 1024;

        boot.kernelModules = [ "mac80211_hwsim" ];
        boot.extraModprobeConfig = ''
          options mac80211_hwsim radios=2
        '';

        environment.systemPackages = [
          self.packages.${system}.default
          pkgs.iwd
          pkgs.tmux
        ];

        networking.firewall.enable = false;
        networking.useDHCP = false;
        networking.wireless.iwd = {
          enable = true;
          settings = {
            General.EnableNetworkConfiguration = true;
            # iwd only honors per-network MAC randomization with this.
            General.AddressRandomization = "network";
            DriverQuirks.UseDefaultInterface = "mac80211_hwsim";
          };
        };

      };

    testScript = ''
      import json

      ${tuiHelpers}

      def list_networks(scan=False, all_networks=False):
          flags = ["--json"]
          if all_networks:
              flags.append("--all")
          if scan:
              flags.append("--scan")
          return json.loads(machine.succeed("NO_COLOR=1 wifitui list " + " ".join(flags))) or []

      def network_by_ssid(networks, ssid):
          for network in networks:
              if network["SSID"] == ssid:
                  return network
          raise AssertionError(f"missing {ssid}: {networks}")

      def wait_for_visible_ssids(required, forbidden=()):
          required = set(required)
          forbidden = set(forbidden)
          networks = []
          for _ in range(8):
              networks = list_networks(scan=True)
              ssids = {network["SSID"] for network in networks}
              if required.issubset(ssids) and forbidden.isdisjoint(ssids):
                  return networks
              machine.sleep(2)
          raise AssertionError(f"wanted {required} without {forbidden}: {networks}")

      start_all()

      machine.wait_for_unit("iwd.service")
      machine.wait_until_succeeds("test -d /sys/class/net/wlan0")
      machine.wait_until_succeeds("test -d /sys/class/net/wlan1")
      machine.succeed("rfkill unblock all")
      machine.succeed("iwctl device wlan0 set-property Mode ap")
      machine.wait_until_succeeds("iwctl device list | grep wlan1")
      machine.wait_until_succeeds("iwctl station wlan1 show | grep -E 'State[[:space:]]+(disconnected|connected)'")
      machine.wait_until_succeeds("iwctl ap list | grep wlan0")
      machine.succeed("iwctl ap wlan0 start Home_Network supersecret")
      machine.wait_until_succeeds("iwctl ap wlan0 show | grep -E 'Started|State[[:space:]]+started'")

      networks = wait_for_visible_ssids(["Home_Network"])

      details = machine.succeed("NO_COLOR=1 wifitui show Home_Network")
      assert "SSID: Home_Network" in details, details
      assert "Secure: true" in details, details
      assert "Visible: true" in details, details

      machine.succeed("NO_COLOR=1 wifitui connect --passphrase supersecret --security wpa --retry-for 30s:2s Home_Network")
      machine.wait_until_succeeds("iwctl station wlan1 show | grep 'Connected network' | grep Home_Network")

      connected = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert connected["IsActive"], connected
      assert connected["IsKnown"], connected

      rescanned = network_by_ssid(list_networks(scan=True, all_networks=True), "Home_Network")
      assert rescanned["IsActive"], rescanned
      assert rescanned["IsKnown"], rescanned

      # Enable MAC randomization for the active network from the TUI, like a
      # user would, and check that it reconnects with a new MAC address.
      def mac_diagnostics():
          return "\n".join([
              "TUI screen:\n" + machine.execute("tmux capture-pane -p -t tui")[1],
              machine.execute("ip link show wlan1")[1],
              machine.execute("iwctl station wlan1 show")[1],
              machine.execute("cat /etc/iwd/main.conf /var/lib/iwd/*.psk")[1],
              machine.execute("journalctl -u iwd --no-pager | tail -n 40")[1],
          ])

      initial_mac = machine.succeed("cat /sys/class/net/wlan1/address").strip()
      randomize_mac_from_tui("Home_Network")
      wait_for_new_mac(
          initial_mac,
          "iwctl station wlan1 show | grep 'Connected network' | grep -q Home_Network",
          mac_diagnostics,
      )
      machine.succeed("grep -qx AlwaysRandomizeAddress=true /var/lib/iwd/Home_Network.psk")

      randomized = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert randomized["RandomizeMAC"], randomized

      machine.succeed("iwctl station wlan1 disconnect")
      machine.wait_until_succeeds("iwctl station wlan1 show | grep -E 'State[[:space:]]+disconnected'")
      disconnected = network_by_ssid(list_networks(all_networks=True), "Home_Network")
      assert not disconnected["IsActive"], disconnected
      assert disconnected["IsKnown"], disconnected

      machine.succeed("iwctl known-networks Home_Network forget")
      forgotten = network_by_ssid(list_networks(scan=True, all_networks=True), "Home_Network")
      assert not forgotten["IsActive"], forgotten
      assert not forgotten["IsKnown"], forgotten
    '';
  };

}
