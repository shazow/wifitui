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
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "wifitui";
          version = "0.0.0"; # Development version is always 0.0.0
          src = ./.;
          # Updated by `make vendorHash`
          vendorHash = "sha256-2smXAK3mRweg0yKDerKgu3fcT3ulDjRSbbkMCSe+nVs=";
          env.CGO_ENABLED = if pkgs.stdenv.isDarwin then "1" else "0";
          ldflags = [
            "-s"
            "-w"
          ];
        };

        checks = import ./checks {
          inherit self system pkgs;
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/wifitui";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.go-tools
          ];
        };
      }
    );
}
