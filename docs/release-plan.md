# Release checklist

## Source preparation

1. Review the standalone Git repository and its current changes.
2. Publish the source at <https://github.com/debugloop/niri-link-router>.
3. Choose the maintainer contact and check the private reporting link in `SECURITY.md`.
4. Keep the MIT license and original-source attribution.
5. Run `nix flake check` and review the native test record.
6. Review the permanent extension ID, version, description, and data-collection declaration.

## Firefox distribution

Follow [Firefox installation and signing](firefox-installation.md).
Use [the AMO submission worksheet](amo-submission.md) for the first listing.
The owner must create the AMO account and initial submission.
The GitHub Actions submission workflow needs protected credentials and a reviewed version tag.
The collector publishes the signed file and Nix metadata after AMO publication.
Manual submission and collection remain available.
Neither path creates an AMO account or guarantees approval.

Publish Mozilla's returned signed XPI unchanged.
Record its fixed file hash for [Nix installation](nix.md).
Do not install the unsigned source archive permanently in Firefox Release.

## Remaining acceptance work

1. Test a signed installation across browser startup, restored windows, native-port disconnects, and reconnection.
2. Test multiple active outputs and output changes.
3. Test physical links from applications other than Kitty.
4. Document any browser packaging whose profile locks or process IDs differ from standard Linux Firefox.
5. Verify native-host registration on supported Firefox packages.

Test background delivery with window urgency while preserving focus.
Use the current native-bridge record for transport evidence. Historical protocol tests do not validate the replacement.
