{
  description = "Forge - Terminal UI client for OpenCode";

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
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gcc
            glibc
            git
          ];
          SHELL = pkgs.fish;
        };

        packages.forge = pkgs.stdenv.mkDerivation {
          pname = "forge";
          version = "0.1.0";
          src = ./.;
          nativeBuildInputs = [ pkgs.go ];
          buildInputs = [ pkgs.gcc pkgs.glibc ];
          buildPhase = ''
            export GOPROXY=https://proxy.golang.org
            export GOSUMDB=sum.golang.org
            export GOWORK=off
            export GOCACHE=$TMPDIR/go-cache
            mkdir -p $GOCACHE
            go build -o $out/bin/forge .
          '';
          installPhase = ''
            # Binary is already built to $out/bin/forge by buildPhase
            :
          '';
        };
      }
    );
}