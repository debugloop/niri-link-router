{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.niri-firefox-router;
  manifest = builtins.fromJSON (builtins.readFile ../extension/manifest.json);
  bgOpen = pkgs.writeShellScriptBin "bg-open" ''
    exec ${lib.getExe' cfg.package "niri-firefox-router"} open --bg "$@"
  '';
in {
  options.programs.niri-firefox-router = import ./options.nix {inherit lib pkgs;};
  config = lib.mkIf cfg.enable (lib.mkMerge [
    {
      home.packages = [cfg.package bgOpen];
      home.file.".mozilla/native-messaging-hosts/niri_firefox_router.json".source = "${cfg.package}/lib/mozilla/native-messaging-hosts/niri_firefox_router.json";
      xdg.configFile."mozilla/native-messaging-hosts/niri_firefox_router.json".source = "${cfg.package}/lib/mozilla/native-messaging-hosts/niri_firefox_router.json";
      xdg.configFile."niri-firefox-router/config.json".text = builtins.toJSON {
        fallback_workspace = cfg.fallbackWorkspace;
      };
      assertions = [
        {
          assertion = cfg.extensionPackage == null || (cfg.extensionPackage.addonId or "") == manifest.browser_specific_settings.gecko.id;
          message = "The router extension package must expose the matching addonId.";
        }
      ];
    }
    (lib.mkIf cfg.defaultBrowser {
      xdg.mimeApps = {
        enable = true;
        defaultApplications = {
          "x-scheme-handler/http" = lib.mkForce ["niri-firefox-router.desktop"];
          "x-scheme-handler/https" = lib.mkForce ["niri-firefox-router.desktop"];
        };
      };
    })
    (lib.mkIf (cfg.extensionPackage != null) {
      programs.firefox = {
        enable = lib.mkDefault true;
        globalExtensions = [
          {
            package = cfg.extensionPackage;
            settings.updates_disabled = true;
          }
        ];
      };
    })
  ]);
}
