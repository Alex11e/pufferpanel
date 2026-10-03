# Publishing a panel release

The Windows publishing helper uploads files that have already been built. It does not compile the panel.

1. Build or collect the release files and place them in `release-assets/` beside `publish-release.bat`.
2. Install GitHub CLI and authenticate with `gh auth login` using an account that can publish releases.
3. Create and push the matching Git tag, for example `git tag v3.1.0` and `git push origin v3.1.0`.
4. Run `publish-release.bat v3.1.0`. The script creates the GitHub release or adds new assets to an existing release. Existing asset names are not overwritten.
5. In the panel, open Admin, choose the release version, and download one of its listed assets.

The updater uses the tagged source archive to rebuild the Docker Compose installation. The release assets are offered as downloads; they are not automatically installed into an unrelated deployment.