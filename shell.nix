{
  mkShell,
  go,
  delve,
  golangci-lint,
  gomod2nix,
  gopls,
  gotools,
  poppler_utils,
}: mkShell {
  packages = [
    go
    delve
    golangci-lint
    gomod2nix
    gopls
    gotools
    poppler_utils
  ];

  hardeningDisable = [ "fortify" ];

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
  '';
}