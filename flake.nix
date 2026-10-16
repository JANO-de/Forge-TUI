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

        packages.default = pkgs.buildGoModule {
          pname = "forge";
          version = "0.1.0";
          src = ./.;
          vendorHash = null; # uses ./vendor
          subPackages = [ "." ];
          nativeCheckInputs = [ pkgs.git ];
          meta.mainProgram = "forge";
        };
        packages.forge = self.packages.${system}.default;
      }
    ) // {
      nixosModules.default = import ./nix/module.nix { inherit self; };
    };
}
