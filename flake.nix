{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "wifitui";
          version = "0.0.0"; # TODO: Add some tags lol
          src = ./.;
          #vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="; # Replace with the actual hash
          vendorHash = "sha256-HZEE8bJC9bsSYmyu7NBoxEprW08DO5+uApVnyNkKgMk=";
          ldflags = [ "-s" "-w" ];
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
      });
}
