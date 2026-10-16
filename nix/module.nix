# NixOS module: add to imports and set `programs.forge.enable = true;`
{ self }:
{ config, lib, pkgs, ... }:
let cfg = config.programs.forge; in
{
  options.programs.forge = {
    enable = lib.mkEnableOption "Forge, a terminal UI for OpenCode";
    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.system}.default;
      description = "Forge package to install.";
    };
  };
  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];
  };
}
