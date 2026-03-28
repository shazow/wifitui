{
  mkSystem = {
    modules = [
      { nixpkgs, lib, ... }: {
        networking.hostName = "wifi-dev";


        # Log in as root automatically on the console
        users.users.root.password = "";
        services.getty.autologinUser = "root";

        # Automatically load the fake Wi-Fi kernel module with 2 radios
        boot.kernelModules = [ "mac80211_hwsim" ];
        boot.extraModprobeConfig = ''
                    options mac80211_hwsim radios=2
        '';

        # Enable the real NetworkManager
        networking.networkmanager = {
          enable = true;
          # Tell NetworkManager to ignore the QEMU wired interface,
          # as well as the wlan0 interface (and its sub-interfaces) 
          # that we are using to broadcast the fake networks.
          unmanaged = [ "eth0" "wlan0" "wlan0_*" ];
        };

        # Declarative HostAPD configuration
        services.hostapd = {
          enable = true;
          radios.wlan0 = {
            band = "2g";
            channel = 6;
            # Define multiple BSS (Basic Service Sets) on the single radio
            networks = {
              wlan0 = {
                ssid = "Airport_Free_WiFi";
              };
              wlan0_0 = {
                ssid = "Home_Network";
                authentication = {
                  mode = "wpa2-sha1";
                  wpaPassword = "supersecret";
                };
              };
              wlan0_1 = {
                ssid = "Test_IoT_Device";
              };
              wlan0_2 = {
                ssid = "Secret_Corp_Net";
                settings = {
                  ignore_broadcast_ssid = "1"; # Hidden network
                };
              };
            };
          };
        };

        # Ensure hostapd doesn't try to start before the kernel module 
        # has finished creating the wlan0 virtual device.
        systemd.services."hostapd-wlan0" = {
          after = [ "sys-subsystem-net-devices-wlan0.device" ];
          bindsTo = [ "sys-subsystem-net-devices-wlan0.device" ];
        };
      }
    ];
  };
}
