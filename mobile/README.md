# Mobile apps (Android + iOS)

Two native apps built with **React Native + Expo** (TypeScript), sharing the same
Go backend as the web apps:

- `mobile/rider` — request a ride, watch the match and trip status live
- `mobile/driver` — go online, stream your location, accept and run trips

Both render a native map (`react-native-maps`) and receive live updates over the
gateway WebSocket.

---

## Prerequisites

- **Node.js 18+** on your computer
- The **backend running** (`make up` in the repo root — see the top-level README)
- A phone with the **Expo Go** app, or an Android/iOS emulator:
  - Expo Go — [Android (Play Store)](https://play.google.com/store/apps/details?id=host.exp.exponent)
    · [iOS (App Store)](https://apps.apple.com/app/expo-go/id982107779)
- Phone and computer on the **same Wi-Fi network**

---

## Run on your phone (fastest — via Expo Go)

Do this for **each** app (`mobile/rider` and `mobile/driver`), in its own terminal.

```bash
cd mobile/rider      # or mobile/driver
npm install          # first time only
npx expo start
```

Then:

1. A QR code appears in the terminal.
2. **Android:** open Expo Go → *Scan QR code*. **iOS:** open the Camera app and
   point it at the QR code, then tap the banner.
3. The app loads on your phone over the LAN.

**Backend connection:** the app auto-detects your computer's LAN IP from the Expo
dev server and talks to the backend at `http://<that-ip>:8080` (REST) and
`ws://<that-ip>:8090` (WebSocket). If detection fails or you run the backend
elsewhere, set the host explicitly in the app's `app.json`:

```json
"extra": { "backendHost": "192.168.1.42" }
```

> The driver app needs **location permission** — grant it when prompted (or it
> falls back to a default map position you can drag).

### Try it end to end

Run **both** apps (two phones, or one phone + one emulator):

1. Driver app → **Go online** (it pings its position every 4s).
2. Rider app → tap the map to set pickup near the driver → **Request ride**.
3. Driver receives the assignment live → **Start** → **Complete**; the rider
   sees each state change stream in.

---

## Run on an emulator

With Android Studio (emulator running) or Xcode (iOS simulator, macOS only):

```bash
cd mobile/rider
npm install
npx expo start
# press "a" for Android emulator, or "i" for iOS simulator
```

On the Android emulator, the backend host auto-resolves; if you need to point at
the host machine manually, use `10.0.2.2` as `backendHost`.

---

## Build installable binaries (APK / IPA)

For a real installable file (no Expo Go) or store distribution, use **EAS Build**
(builds in the cloud — no local Android/Xcode toolchain required):

```bash
npm install -g eas-cli
eas login
cd mobile/rider          # repeat for mobile/driver
eas build --platform android   # produces an .apk/.aab
eas build --platform ios       # produces an .ipa (needs an Apple Developer account)
```

Notes for standalone builds:

- **Android Google Maps:** `react-native-maps` needs a Google Maps API key on
  Android. Add it to `app.json` → `android.config.googleMaps.apiKey`. (iOS uses
  Apple Maps, no key needed.) Expo Go works without a key for development.
- **Cleartext HTTP:** the demo backend is plain HTTP. Expo Go allows this in
  development; a production Android build blocks cleartext traffic by default, so
  point the apps at an HTTPS backend (or enable cleartext explicitly) before
  shipping.

See the Expo docs for distribution details: https://docs.expo.dev/build/introduction/
