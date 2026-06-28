import { DEFAULT_CENTER } from "../shared/config.js";
import { postJSON, getJSON, randomId } from "../shared/api.js";
import { connectLive } from "../shared/live.js";

const $ = (id) => document.getElementById(id);

let riderId = randomId("rider");
$("riderId").value = riderId;

// --- map ---
const map = L.map("map").setView(DEFAULT_CENTER, 14);
L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
  attribution: "© OpenStreetMap",
  maxZoom: 19,
}).addTo(map);

const pickup = L.marker(DEFAULT_CENTER, { draggable: true }).addTo(map);
pickup.bindTooltip("Pickup", { permanent: false });
let driverMarker = null;

function pos() {
  const { lat, lng } = pickup.getLatLng();
  return { lat, lng };
}

function refreshPickup() {
  const { lat, lng } = pos();
  $("lat").textContent = lat.toFixed(5);
  $("lng").textContent = lng.toFixed(5);
  refreshSurge();
}

map.on("click", (e) => {
  pickup.setLatLng(e.latlng);
  refreshPickup();
});
pickup.on("dragend", refreshPickup);

// --- surge ---
async function refreshSurge() {
  try {
    const { lat, lng } = pos();
    const s = await getJSON(`/v1/surge?lat=${lat}&lng=${lng}`);
    $("surge").textContent = s.surge.toFixed(2);
    $("supply").textContent = s.supply;
  } catch {
    /* backend not up yet */
  }
}
setInterval(refreshSurge, 5000);

// --- live notifications ---
let live = null;
function connect() {
  live && live.close();
  live = connectLive(riderId, {
    onStatus: (st) => {
      const el = $("conn");
      el.textContent = st === "connected" ? "live" : "offline";
      el.className = "pill " + (st === "connected" ? "on" : "off");
    },
    onMessage: (n) => {
      logEvent(n.message);
      if (n.kind && n.kind.startsWith("trip:")) {
        setState(n.kind.slice("trip:".length));
      }
    },
  });
}

$("riderId").addEventListener("change", (e) => {
  riderId = e.target.value.trim() || randomId("rider");
  $("riderId").value = riderId;
  connect();
});

// --- trip ---
let tripId = null;

function setState(state) {
  $("tripCard").style.display = "block";
  $("state").textContent = state;
  $("cancel").disabled = state === "COMPLETED" || state === "CANCELLED";
}

$("request").addEventListener("click", async () => {
  const { lat, lng } = pos();
  try {
    const r = await postJSON("/v1/rides", { rider_id: riderId, lat, lng });
    tripId = r.trip.id;
    setState(r.trip.state);
    $("surge").textContent = (r.surge ?? 1).toFixed(2);
    if (r.matched) {
      $("driver").textContent = r.driver.driver_id;
      $("dist").textContent = `${r.driver.distance_km.toFixed(2)} km`;
      showDriver(r.driver);
      logEvent(`Matched ${r.driver.driver_id} (${r.shortlist.length} nearby)`);
    } else {
      $("driver").textContent = "—";
      $("dist").textContent = "—";
      logEvent(r.message || "No driver available");
    }
  } catch (err) {
    logEvent("Request failed: " + err.message);
  }
});

$("cancel").addEventListener("click", async () => {
  if (!tripId) return;
  try {
    await postJSON(`/v1/trips/${tripId}/cancel`);
  } catch (err) {
    logEvent("Cancel failed: " + err.message);
  }
});

function showDriver(d) {
  const ll = [d.lat, d.lng];
  if (driverMarker) driverMarker.setLatLng(ll);
  else {
    driverMarker = L.circleMarker(ll, {
      radius: 9,
      color: "#2d7ff9",
      fillColor: "#2d7ff9",
      fillOpacity: 0.9,
    }).addTo(map);
    driverMarker.bindTooltip("Your driver");
  }
  map.fitBounds(L.latLngBounds([pickup.getLatLng(), ll]).pad(0.5));
}

// --- log ---
function logEvent(msg) {
  const e = document.createElement("div");
  e.className = "entry";
  const t = new Date().toLocaleTimeString();
  e.innerHTML = `<time>${t}</time>${msg}`;
  $("log").appendChild(e);
}

refreshPickup();
connect();
