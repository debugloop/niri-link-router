{pkgs}: let
  inherit (pkgs) lib;
  bool = default:
    lib.mkOption {
      type = lib.types.bool;
      inherit default;
    };
  packages = lib.mkOption {
    type = lib.types.listOf lib.types.package;
    default = [];
  };
  attrs = lib.mkOption {
    type = lib.types.attrs;
    default = {};
  };
  associations = lib.mkOption {
    type = lib.types.attrsOf (lib.types.listOf lib.types.str);
    default = {};
  };
  assertions = lib.mkOption {
    type = lib.types.listOf lib.types.attrs;
    default = [];
  };
  addonId = (builtins.fromJSON (builtins.readFile ../extension/manifest.json)).browser_specific_settings.gecko.id;
  fixture =
    pkgs.writeShellScriptBin "niri-firefox-router" ''
      printf '%s\n' "$@"
    ''
    // {inherit addonId;};
  hm = lib.evalModules {
    specialArgs = {inherit pkgs;};
    modules = [
      ./home-manager.nix
      {
        options = {
          inherit assertions;
          home.packages = packages;
          home.file = attrs;
          xdg.configFile = attrs;
          xdg.mimeApps = {
            enable = bool false;
            defaultApplications = associations;
          };
          programs.firefox = {
            enable = bool false;
            globalExtensions = lib.mkOption {
              type = lib.types.listOf (lib.types.submodule {
                options.package = lib.mkOption {type = lib.types.package;};
                options.settings = attrs;
              });
              default = [];
            };
            profiles = attrs;
          };
        };
        config = {
          programs.niri-firefox-router = {
            enable = true;
            package = fixture;
            defaultBrowser = true;
            extensionPackage = fixture;
            fallbackWorkspace = "browser";
          };
          programs.firefox.profiles.existing.id = 0;
          xdg.mimeApps.defaultApplications."x-scheme-handler/mailto" = ["mail.desktop"];
        };
      }
    ];
  };
  os = lib.evalModules {
    specialArgs = {inherit pkgs;};
    modules = [
      ./nixos.nix
      {
        options = {
          inherit assertions;
          environment.systemPackages = packages;
          environment.etc = attrs;
          xdg.mime = {
            enable = bool false;
            defaultApplications = associations;
          };
          programs.firefox = {
            enable = bool false;
            policies = attrs;
            nativeMessagingHosts.packages = packages;
          };
        };
        config.programs.niri-firefox-router = {
          enable = true;
          package = fixture;
          defaultBrowser = true;
          extensionPackage = fixture;
          fallbackWorkspace = "internet";
        };
      }
    ];
  };
in
  assert hm.config.programs.firefox.profiles.existing.id == 0;
  assert (builtins.head hm.config.programs.firefox.globalExtensions).package == fixture;
  assert (builtins.head hm.config.programs.firefox.globalExtensions).settings.updates_disabled;
  assert map lib.getName hm.config.home.packages == ["niri-firefox-router" "bg-open"];
  assert hm.config.home.file.".mozilla/native-messaging-hosts/niri_firefox_router.json".source == "${fixture}/lib/mozilla/native-messaging-hosts/niri_firefox_router.json";
  assert hm.config.xdg.configFile."mozilla/native-messaging-hosts/niri_firefox_router.json".source == "${fixture}/lib/mozilla/native-messaging-hosts/niri_firefox_router.json";
  assert (builtins.fromJSON hm.config.xdg.configFile."niri-firefox-router/config.json".text).fallback_workspace == "browser";
  assert hm.config.xdg.mimeApps.defaultApplications."x-scheme-handler/http" == ["niri-firefox-router.desktop"];
  assert hm.config.xdg.mimeApps.defaultApplications."x-scheme-handler/mailto" == ["mail.desktop"];
  assert builtins.all (a: a.assertion) hm.config.assertions;
  assert os.config.environment.systemPackages == [fixture];
  assert os.config.programs.firefox.nativeMessagingHosts.packages == [fixture];
  assert (builtins.fromJSON os.config.environment.etc."niri-firefox-router/config.json".text).fallback_workspace == "internet";
  assert os.config.xdg.mime.defaultApplications."x-scheme-handler/https" == ["niri-firefox-router.desktop"];
  assert os.config.programs.firefox.policies.ExtensionSettings.${addonId}.installation_mode == "force_installed";
  assert os.config.programs.firefox.policies.ExtensionSettings.${addonId}.updates_disabled;
  assert builtins.all (a: a.assertion) os.config.assertions;
    pkgs.runCommand "router-module-check" {} ''
      url='https://example.org/a+b?q=%25&mode=bg#fragment'
      expected=$(printf '%s\n' open --bg "$url")
      actual=$(${lib.getExe' (builtins.elemAt hm.config.home.packages 1) "bg-open"} "$url")
      test "$actual" = "$expected"
      touch "$out"
    ''
