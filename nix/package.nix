{pkgs}: let
  manifest = builtins.fromJSON (builtins.readFile ../extension/manifest.json);
in
  pkgs.buildGoModule {
    pname = "niri-firefox-router";
    inherit (manifest) version;
    src = pkgs.lib.cleanSource ../.;
    vendorHash = null;
    subPackages = ["cmd/niri-firefox-router" "cmd/niri-firefox-router-native-host"];
    env = {
      CGO_ENABLED = 0;
      GOTOOLCHAIN = "local";
    };
    nativeCheckInputs = with pkgs; [nodejs python3 bash xdg-utils];
    checkPhase = ''
      runHook preCheck
      go test ./...
      go vet ./...
      node --test tests/extension.test.cjs
      python3 tests/cli_smoke.py
      python3 -m unittest discover -s tests -p 'test_*.py'
      runHook postCheck
    '';
    postInstall = ''
      mkdir -p $out/lib/mozilla/native-messaging-hosts
      cat > $out/lib/mozilla/native-messaging-hosts/niri_firefox_router.json <<EOF_NATIVE
      {"name":"niri_firefox_router","description":"${manifest.name} native bridge","path":"$out/bin/niri-firefox-router-native-host","type":"stdio","allowed_extensions":["${manifest.browser_specific_settings.gecko.id}"]}
      EOF_NATIVE
      mkdir -p $out/share/applications
      cat > $out/share/applications/niri-firefox-router.desktop <<EOF_DESKTOP
      [Desktop Entry]
      Type=Application
      Name=${manifest.name}
      Exec=$out/bin/niri-firefox-router open %u
      Terminal=false
      NoDisplay=true
      MimeType=x-scheme-handler/http;x-scheme-handler/https;
      EOF_DESKTOP
    '';
    meta = {
      description = "Open external links in Firefox on the current or configured fallback niri workspace";
      license = pkgs.lib.licenses.mit;
      platforms = pkgs.lib.platforms.linux;
      mainProgram = "niri-firefox-router";
    };
  }
