{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.niri-firefox-router;
  manifest = builtins.fromJSON (builtins.readFile ../extension/manifest.json);
  addonId = manifest.browser_specific_settings.gecko.id;
in {
  options.programs.niri-firefox-router = import ./options.nix {inherit lib pkgs;};
  config = lib.mkIf cfg.enable (lib.mkMerge [
    {
      environment.systemPackages = [cfg.package];
      programs.firefox = {
        enable = lib.mkDefault true;
        nativeMessagingHosts.packages = [cfg.package];
      };
      environment.etc."niri-firefox-router/config.json".text = builtins.toJSON {
        fallback_workspace = cfg.fallbackWorkspace;
      };
      assertions = [
        {
          assertion = cfg.extensionPackage == null || (cfg.extensionPackage.addonId or "") == addonId;
          message = "The router extension package must expose the matching addonId.";
        }
      ];
    }
    (lib.mkIf cfg.defaultBrowser {
      xdg.mime = {
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
        policies.ExtensionSettings.${addonId} = {
          installation_mode = "force_installed";
          updates_disabled = true;
          install_url = "file://${cfg.extensionPackage}/share/mozilla/extensions/{ec8030f7-c20a-464f-9b0e-13a3a9e97384}/${addonId}.xpi";
        };
      };
    })
  ]);
}
