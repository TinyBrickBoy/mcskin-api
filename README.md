# Build-Artefakte

Fertige `mcskins`-Binaries, automatisch pro Branch gebaut.

Hol dir deine Binary z.B. so:

```bash
git clone --branch builds --single-branch <repo-url> mcskins-builds
# oder im bestehenden Klon:
git fetch origin builds && git checkout builds && git pull
```

Verzeichnis = Quell-Branch, Datei = mcskins-<os>-<arch>. Checksummen in SHA256SUMS.txt.
