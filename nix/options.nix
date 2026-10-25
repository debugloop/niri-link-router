{
  lib,
  pkgs,
}: {
  enable = lib.mkEnableOption "niri Firefox URL routing";
  package = lib.mkOption {
    type = lib.types.package;
    default = import ./package.nix {inherit pkgs;};
    description = "The router command and desktop entry.";
  };
  fallbackWorkspace = lib.mkOption {
    type = lib.types.nonEmptyStr;
    default = "web";
    description = "The named workspace to use when the current workspace has no Firefox window.";
  };
  defaultBrowser = lib.mkOption {
    type = lib.types.bool;
    default = false;
    description = "Use the router for HTTP and HTTPS links.";
  };
  extensionPackage = lib.mkOption {
    type = lib.types.nullOr lib.types.package;
    default = null;
    description = "An optional Mozilla-signed extension package with addonId metadata. Never use the unsigned source archive for Firefox Release.";
  };
}
