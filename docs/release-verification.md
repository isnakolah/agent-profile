# Release verification

Implementation and automated tests are distinct from provider and host acceptance. Do not mark the release gate complete from a green unit-test run alone.

## Automated acceptance

Run the macOS/Linux CI matrix. It covers registry migration, concurrent writers, unsafe paths, launcher flags and collisions, literal arguments, auth environment replacement, native PTY launch, detach/reattach, viewers, control transfer, resize, exit status, interrupted metadata, and protocol bounds. Run the optional OS-vault test on each host with an unlocked vault.

## Provider and host acceptance

Use two separately authenticated profiles. For both Codex and Claude:

1. Start the native interface through the profile command and confirm the intended account using the provider's own account display.
2. Start a second profile and verify logout or credential rotation in one does not change the other.
3. Submit a small, explicit test request; close the terminal while it is working, then reattach and verify the same conversation and result.
4. Attach another terminal as a viewer, verify it cannot type, then transfer control. Test resize, Unicode, copy/paste and native shortcuts.
5. Repeat with an API-key profile. Verify the vault is required and manager metadata contains no key.
6. Reboot the test host, sign in, select the interrupted entry and resume the intended conversation through the provider picker. Confirm the original prompt was not replayed.
7. Verify status-service startup and native notification delivery on macOS and Linux.

Record host versions, exact installed commit/tag, provider versions, results and any limitations in the GitHub issue evidence. Interactive login and a real reboot require operator participation. An unavailable GUI automation surface is not visual acceptance.

## Publishing

Initialize the `Implementation-Log` Wiki page if needed. Reconcile accepted GFD evidence, link the implementation PR and its exact final SHA, and verify all acceptance boundaries before closing stories or parent epics. Publish `v0.1.0` only after the release acceptance above passes. Keep release candidates clearly labeled while any host/provider gate remains open.
