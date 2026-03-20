{
  nixConfig.extra-experimental-features = [
    "pipe-operators"
    "no-url-literals"
  ];

  inputs = {
    nixpkgs.url = "https://channels.nixos.org/nixos-unstable/nixexprs.tar.xz";

    kasumi = {
      url = "https://codeberg.org/api/v1/repos/nadevko/kasumi/archive/cc0a6826be2c4c4c6a419d7b420980b5d58bebca.tar.gz";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      kasumi,
      gomod2nix,
    }:
    let
      so = self.overlays;
      k = kasumi.lib;
      ko = kasumi.overlays;
    in
    {
      apps = k.forAllPkgs self { } (pkgs: {
        swagen = {
          type = "app";
          program =
            toString
            <| pkgs.writeScript "swagen" ''
              #!/usr/bin/env bash
              set -euo pipefail
              cd "${self}/api"
              ${pkgs.go-swag}/bin/swag init -g doc.go -d ./internal/api -o docs
              if [[ -f docs/swagger.json ]]; then
                mv -f docs/swagger.json docs/v1-alpha.json
              fi
            '';
        };

        tunnel = {
          type = "app";
          program =
            toString
            <| pkgs.writeScript "tunnel" ''
              #!/usr/bin/env bash

              cleanup() {
                  kill $(jobs -p) 2>/dev/null
              }
              trap cleanup EXIT SIGINT SIGTERM

              cd "${self}/api"
              ${pkgs.go-swag}/bin/swag init -g doc.go -d ./internal/api -o docs
              if [[ -f docs/swagger.json ]]; then
                mv -f docs/swagger.json docs/v1-alpha.json
              fi

              cloudflared tunnel run --token "$CLOUDFLARE_TOKEN" &
              go run ./cmd/legist-api/main.go
            '';
        };
      });

      nixosModules = rec {
        default = legist-api;
        legist-api = ./api/nix/nixos-module.nix;
      };

      overlays = {
        default = import ./api/nix/overlay.nix;
        augment = k.augmentLib ko.lib;

        environment = k.foldLay [
          ko.compat
          ko.default
          gomod2nix.overlays.default
          so.augment
        ];
      };

      packages = k.forAllPkgs nixpkgs { } (
        pkgs:
        pkgs
        |> k.makeLayer so.environment
        |> k.rebaseLayerTo so.default
        |> k.collapseSupportedBy pkgs.stdenv.hostPlatform.system
      );

      legacyPackages = k.importPkgsForAll nixpkgs {
        overlays = [
          so.environment
          so.default
        ];
      };

      devShells = k.forAllPkgs self { } (pkgs: rec {
        default = legist-api-shell;
        legist-api-shell = pkgs.callPackage ./api/shell.nix { };
      });

      formatter = k.forAllPkgs self { } <| builtins.getAttr "kasumi-fmt";
      templates.rfc.path = ./docs/rfcs/0000-template.md;
    };
}
