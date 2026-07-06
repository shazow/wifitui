{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        lib = pkgs.lib;
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "wifitui";
          version = "0.0.0"; # Development version is always 0.0.0
          src = ./.;
          # Updated by `make vendorHash`
          vendorHash = "sha256-2smXAK3mRweg0yKDerKgu3fcT3ulDjRSbbkMCSe+nVs=";
          ldflags = [
            "-s"
            "-w"
          ];
        };

        checks = lib.optionalAttrs pkgs.stdenv.isLinux {
          networkmanager-hwsim = pkgs.nixosTest {
            name = "wifitui-networkmanager-hwsim";

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
                  pkgs.jq
                  pkgs.iw
                  pkgs.iproute2
                  pkgs.networkmanager
                ];

                networking.firewall.enable = false;

                networking.networkmanager = {
                  enable = true;
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
              };

            testScript = ''
              import json

              def list_networks(scan=False, all_networks=False):
                  flags = ["--json"]
                  if all_networks:
                      flags.append("--all")
                  if scan:
                      flags.append("--scan")
                  return json.loads(machine.succeed("NO_COLOR=1 wifitui list " + " ".join(flags)))

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
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/wifitui";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.golint
          ];
        };
      }
    );
}
