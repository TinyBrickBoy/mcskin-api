# mcskins

Eine schnelle, schlanke Minecraft-Skin-API in Go — **keine externen Dependencies**, nur die Standard-Library.

Sie löst Minecraft-Benutzernamen oder UUIDs über die offiziellen Mojang-APIs auf, holt die Skin-Textur und rendert daraus Avatare (Gesicht, Kopf mit Hut-Layer, Ganzkörper). Es wird **nichts zwischengespeichert** — jede Anfrage fragt Mojang frisch ab.

## Endpoints

| Methode & Pfad              | Beschreibung                                        |
| --------------------------- | --------------------------------------------------- |
| `GET /skin/{player}`        | Rohe Skin-Textur (PNG, 64×64)                       |
| `GET /face/{player}`        | Gesicht (8×8 Region), skaliert                      |
| `GET /head/{player}`        | Gesicht inkl. Hut-/Overlay-Layer                    |
| `GET /avatar/{player}`      | Alias für `/head`                                   |
| `GET /body/{player}`        | Flacher Ganzkörper-Render von vorne                 |
| `GET /pfp/{player}`         | Stilisierter 20×20 Büsten-Avatar (Kopf + Schultern) |
| `GET /3dpfp/{player}`       | 3D-Render mit großem Kopf (fester Winkel)           |
| `GET /health`               | Healthcheck (`{"status":"ok"}`)                     |
| `GET /`                     | API-Übersicht als JSON                              |

- `{player}` ist ein **Benutzername oder eine UUID** (mit oder ohne Bindestriche).
- Bildgröße per Query: `?size=N` (1–512, Standard 128). Skaliert mit Nearest-Neighbor → scharfe Pixel.
- Spieler ohne eigenen Skin bekommen einen deterministischen Fallback-Skin (kein Fehler).
- `/3dpfp` rendert ein „Big-Head"-3D-Modell mit einem reinen Software-Rasterizer (Z-Buffer,
  Supersampling, perspektivische Projektion) — ohne GPU und ohne externe Dependencies. Der
  Kamerawinkel ist fest, damit alle Avatare gleich aussehen; nur `?size=` ist einstellbar.

### Beispiele

```bash
curl localhost:8080/face/Notch?size=256 -o notch.png
curl localhost:8080/head/jeb_         -o jeb.png
curl localhost:8080/body/Notch        -o notch_body.png
curl localhost:8080/pfp/Notch?size=200 -o notch_pfp.png
curl localhost:8080/3dpfp/Notch?size=400 -o notch_3dpfp.png
curl localhost:8080/skin/069a79f4-44e9-4726-a5be-fca90e38aaf5 -o skin.png
```

## Starten

```bash
go run ./cmd/mcskins
# oder
go build -o mcskins ./cmd/mcskins && ./mcskins
```

### Konfiguration (Umgebungsvariablen)

| Variable                     | Standard | Bedeutung                                         |
| ---------------------------- | -------- | ------------------------------------------------- |
| `MCSKINS_ADDR`               | `:8080`  | Listen-Adresse                                    |
| `MCSKINS_PROXIES`            | _(leer)_ | Komma-Liste von Proxy-URLs für Rate-Limit-Fallback |

## Proxy-Netzwerk (Rate-Limit-Fallback)

Mojang limitiert pro IP. Damit der Dienst trotzdem durchhält, kann jeder Knoten
ein **eigenes Proxy-Netzwerk** nutzen: Jede Anfrage geht zuerst **direkt** raus;
antwortet Mojang mit `HTTP 429`, wird **dieselbe** Anfrage über die konfigurierten
Proxys erneut versucht — der Reihe nach, bis einer Budget hat. Jeder Proxy hat eine
eigene IP, also ein eigenes Rate-Limit-Budget. Sind alle erschöpft, gibt die API
`429` zurück.

```bash
# erst direkt, dann über zwei SOCKS5-Proxys, dann über einen HTTP-Proxy
export MCSKINS_PROXIES="socks5://10.0.0.2:1080,socks5://user:pass@10.0.0.3:1080,http://10.0.0.4:3128"
go run ./cmd/mcskins
```

Unterstützt werden alle Schemata von Go's `net/http`: `socks5`, `http`, `https`.
Nicht parsebare Einträge werden ignoriert. **Keine externen Dependencies** nötig.

## Fertige Binaries (CI-Builds)

Jeder Push baut per GitHub Actions installierbare Binaries für Linux, macOS und
Windows (amd64/arm64). Du kommst auf zwei Wegen dran:

1. **Direkt herunterladen:** Im jeweiligen Actions-Run unter _Artifacts_
   (`mcskins-binaries-*`).
2. **Per Git ziehen:** Alle Builds liegen im Branch `builds`, sortiert nach
   Quell-Branch:

   ```bash
   git clone --branch builds --single-branch <repo-url> mcskins-builds
   # Binary liegt unter <quell-branch>/mcskins-<os>-<arch>
   chmod +x mcskins-builds/<quell-branch>/mcskins-linux-amd64
   ```

   Checksummen stehen in `SHA256SUMS.txt`.

## Docker

```bash
docker build -t mcskins .
docker run -p 3000:3000 mcskins
```

Das Image lauscht auf **Port 3000** (`MCSKINS_ADDR=:3000`). Für einen anderen Port
einfach überschreiben: `docker run -p 8080:8080 -e MCSKINS_ADDR=:8080 mcskins`.

## Tests

```bash
go test ./...
```

## Architektur

```
cmd/mcskins        Einstiegspunkt, Server-Lifecycle, Graceful Shutdown
internal/server    HTTP-Routing, Handler, Fehler-Mapping, Logging
internal/mojang    Mojang-Client: Name→UUID→Textur, Proxy-Fallback, Fallback-Skin
internal/render    Ausschneiden & Skalieren von Skin-Regionen zu PNGs
```
