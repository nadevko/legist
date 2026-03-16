{
  mkShell,
  go,
  delve,
  golangci-lint,
  gomod2nix,
  gopls,
  gotools,
}:
mkShell {
  packages = [
    go
    delve
    golangci-lint
    gomod2nix
    gopls
    gotools
  ];

  GOPATH = "${toString ./.}/.go";
  GOROOT = "${go}/share/go";

  shellHook = ''
    export PATH="$GOPATH/bin:$PATH"
    echo "Go environment is ready. Go version: $(go version)"
  '';

  templates.rfc = ./docs/rfcs/0000-template.md;
}
