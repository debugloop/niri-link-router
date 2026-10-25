{
  description = "Workspace-aware Firefox URL routing for niri";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    pre-commit-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs @ {
    self,
    nixpkgs,
    ...
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux"];
    eachPkgs = function:
      nixpkgs.lib.genAttrs supportedSystems
      (system: function nixpkgs.legacyPackages.${system});
    eachPkgsSystem = function:
      nixpkgs.lib.genAttrs supportedSystems
      (system: function nixpkgs.legacyPackages.${system} system);
    mkPackage = pkgs: import ./nix/package.nix {inherit pkgs;};
    manifest = builtins.fromJSON (builtins.readFile ./extension/manifest.json);
    addonId = manifest.browser_specific_settings.gecko.id;
  in {
    packages = eachPkgs (pkgs: {
      default = mkPackage pkgs;
      firefox-extension-unsigned =
        pkgs.runCommand "niri-firefox-router-${manifest.version}-unsigned.xpi" {
          nativeBuildInputs = [pkgs.zip];
        } ''
          cp -r ${./extension} extension
          chmod -R u+w extension
          cd extension
          zip -X -r "$out" .
        '';
    });

    apps = eachPkgsSystem (pkgs: system: {
      default = {
        type = "app";
        meta.description = "Open a URL in the appropriate Firefox window";
        program = pkgs.lib.getExe self.packages.${system}.default;
      };
    });

    checks = eachPkgsSystem (pkgs: system: {
      router = self.packages.${system}.default;
      modules = import ./nix/module-check.nix {inherit pkgs;};
      workflows =
        pkgs.runCommand "niri-firefox-router-workflow-check" {
          nativeBuildInputs = [pkgs.actionlint pkgs.shellcheck];
        } ''
          actionlint ${./.github/workflows}/*.yml
          touch "$out"
        '';
      extension =
        pkgs.runCommand "niri-firefox-router-extension-check" {
          nativeBuildInputs = [pkgs.web-ext pkgs.unzip];
        } ''
          web-ext lint --source-dir ${./extension} --warnings-as-errors
          unzip -t ${self.packages.${system}.firefox-extension-unsigned}
          touch "$out"
        '';
      pre-commit-check = inputs.pre-commit-hooks.lib.${system}.run {
        src = ./.;
        hooks = {
          alejandra = {
            enable = true;
            name = "nix lint";
            types = ["nix"];
          };
          golangci-lint = {
            enable = true;
            package = pkgs.writeShellScriptBin "golangci-lint" ''
              export PATH="${pkgs.go}/bin''${PATH:+:$PATH}"
              export GOTOOLCHAIN=local
              exec ${pkgs.golangci-lint}/bin/golangci-lint "$@"
            '';
            types = ["go"];
            excludes = ["^vendor/"];
          };
        };
      };
    });

    devShells = eachPkgsSystem (pkgs: system: {
      default = pkgs.mkShell {
        buildInputs = self.checks.${system}.pre-commit-check.enabledPackages;
        packages = with pkgs; [go delve go-tools golangci-lint gopls gotestsum gotools nodejs python3 web-ext];
        shellHook = ''
          ${self.checks.${system}.pre-commit-check.shellHook}
          export GOPATH="$(realpath .)/.cache/go"
          export GOCACHE="$(realpath .)/.cache/go-build"
          export PATH="$GOPATH/bin:$PATH"
        '';
        env = {
          GOROOT = "${pkgs.go}/share/go";
          GOTOOLCHAIN = "local";
        };
        hardeningDisable = ["fortify"];
      };
    });

    formatter = eachPkgs (pkgs:
      pkgs.writeShellScriptBin "nix-fmt" ''
        if [ "$#" -eq 0 ]; then set -- .; fi
        exec ${pkgs.alejandra}/bin/alejandra "$@"
      '');
    nixosModules.default = import ./nix/nixos.nix;
    homeManagerModules.default = import ./nix/home-manager.nix;

    lib.mkSignedExtension = {
      pkgs,
      url,
      hash,
    }:
      pkgs.runCommand "niri-firefox-router-${manifest.version}-signed" {
        src = pkgs.fetchurl {inherit url hash;};
        passthru = {inherit addonId;};
        meta = {
          license = pkgs.lib.licenses.mit;
          mozPermissions = ["storage" "nativeMessaging"];
        };
      } ''
        target="$out/share/mozilla/extensions/{ec8030f7-c20a-464f-9b0e-13a3a9e97384}/${addonId}.xpi"
        install -Dm444 "$src" "$target"
        cmp "$src" "$target"
      '';
  };
}
