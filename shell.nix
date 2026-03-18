{
  mkShell,
  go,
  delve,
  golangci-lint,
  gomod2nix,
  gopls,
  gotools,
  go-swag,
}:
mkShell {
  packages = [
    go
    delve
    golangci-lint
    gomod2nix
    gopls
    gotools
    go-swag
  ];

  hardeningDisable = [ "fortify" ];

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
  '';
}
