final: prev: rec {
  default = legist-api;
  legist-api = final.callPackage ./legist-api.nix { };
  legist-api-image = final.callPackage ./legist-api-image.nix { };
}
