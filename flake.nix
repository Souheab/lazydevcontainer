{
  description = "Nix flake for lazydc";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        lib = pkgs.lib;
        go = if pkgs ? go_1_24 then pkgs.go_1_24 else pkgs.go;
        buildGoModule =
          if pkgs ? buildGo124Module then
            pkgs.buildGo124Module
          else
            pkgs.buildGoModule.override { inherit go; };

        src =
          assert !builtins.pathExists ./vendor;
          lib.cleanSourceWith {
            src = ./.;
            filter = path: type:
              lib.cleanSourceFilter path type && builtins.baseNameOf path != "vendor";
          };

        lazydc = buildGoModule {
          pname = "lazydc";
          version = "0.0.0";
          inherit src;
          subPackages = [ "cmd/lazydc" ];
          vendorHash = "sha256-egPRt6RhS96bDtwpRIRW7W+42ZPxfo/7wQUSN+pjmWc=";
          ldflags = [ "-s" "-w" ];

          meta = {
            mainProgram = "lazydc";
            description = "Terminal UI for managing dev containers";
          };
        };
      in
      {
        packages.default = lazydc;
        packages.lazydc = lazydc;

        devShells.default = pkgs.mkShell {
          packages = [
            go
            pkgs.gopls
          ];

          GOFLAGS = "-mod=mod";

          shellHook = ''
            unset GOROOT
          '';
        };

        formatter = pkgs.nixfmt-rfc-style;
      });
}