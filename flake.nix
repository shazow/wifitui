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
                  pkgs.networkmanager
                ];

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
              };

            testScript = ''
              import json

              start_all()

              machine.wait_for_unit("NetworkManager.service")
              machine.wait_until_succeeds("test -d /sys/class/net/wlan0")
              machine.wait_until_succeeds("test -d /sys/class/net/wlan1")
              machine.wait_for_unit("hostapd.service")
              machine.succeed("rfkill unblock all")
              machine.succeed("nmcli radio wifi on")
              machine.wait_until_succeeds("nmcli -t -f DEVICE,TYPE device | grep '^wlan1:wifi$'")
              machine.wait_until_succeeds("nmcli -t -f DEVICE,STATE device | grep -E '^wlan1:(disconnected|connected)$'")

              output = machine.succeed("NO_COLOR=1 wifitui list --json --scan")
              ssids = {network["SSID"] for network in json.loads(output)}
              assert "Airport_Free_WiFi" in ssids, output
              assert "Home_Network" in ssids, output
              assert "Test_IoT_Device" in ssids, output
              assert "Secret_Corp_Net" not in ssids, output

              details = machine.succeed("NO_COLOR=1 wifitui show Home_Network")
              assert "SSID: Home_Network" in details, details
              assert "Secure: true" in details, details
              assert "Visible: true" in details, details
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
