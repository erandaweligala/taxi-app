import { DEFAULT_CENTER } from "../shared/config.js";
import { postJSON, randomId } from "../shared/api.js";
import { connectLive } from "../shared/live.js";

const $ = (id) => document.getElementById(id);

let driverId = randomId("driver");
$("driverId").value = driverId;

// --- map ---
const map = L.map("map").setView(DEFAULT_CENTER, 14);
L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
  attribution: "© OpenStreetMap",
  maxZoom: 19,
}).addTo(map);

const me = L.marker(DEFAULT_CENTER, { draggable: true }).addTo(map);
me.bindTooltip("You");

function pos() {
  const { lat, lng } = me.getLatLng();
  return { lat, lng };
}
function refreshPos() {
  const { lat, lng } = pos();
  $("posn").textContent = `${lat.toFixed(4)}, ${lng.toFixed(4)}`;
}
me.on("dragend", () => {
  refreshPos();
  if (online) ping(); // send updated position immediately
});

// --- online / pinging ---
let online = false;
let timer = null;

async function ping() {
  const { lat, lng } = pos();
  try {
    await postJSON(`/v1/drivers/${driverId}/location`, {
      lat,
      lng,
      available: online,
    });
    $("ping").textContent = new Date().toLocaleTimeString();
  } catch (err) {
    logEvent("Ping failed: " + err.message);
  }
}

function setAvail(on) {
  online = on;
  const el = $("avail");
  el.textContent = on ? "online" : "offline";
  el.className = "pill " + (on ? "on" : "off");
  $("toggle").textContent = on ? "Go offline" : "Go online";
  $("toggle").className = on ? "danger" : "secondary";
}

$("toggle").addEventListener("click", async () => {
  if (online) {
    setAvail(false);
    clearInterval(timer);
    timer = null;
    await ping(); // final ping marks the driver unavailable
  } else {
    setAvail(true);
    await ping();
    timer = setInterval(ping, 4000);
  }
});

// --- live notifications / assignment ---
let live = null;
let tripId = null;

function connect() {
  live && live.close();
  live = connectLive(driverId, {
    onStatus: (st) => {
      const el = $("conn");
      el.textContent = st === "connected" ? "live" : "offline";
      el.className = "pill " + (st === "connected" ? "on" : "off");
    },
    onMessage: (n) => {
      logEvent(n.message);
      handle(n);
    },
  });
}

function handle(n) {
  if (!n.kind || !n.kind.startsWith("trip:")) return;
  const state = n.kind.slice("trip:".length);
  if (state === "ASSIGNED") {
    tripId = n.trip_id;
    $("tripCard").style.display = "block";
    $("tripId").textContent = tripId;
    $("state").textContent = "ASSIGNED";
    $("start").disabled = false;
    $("complete").disabled = true;
  } else {
    $("state").textContent = state;
  }
}

$("start").addEventListener("click", async () => {
  if (!tripId) return;
  try {
    const t = await postJSON(`/v1/trips/${tripId}/start`);
    $("state").textContent = t.state;
    $("start").disabled = true;
    $("complete").disabled = false;
  } catch (err) {
    logEvent("Start failed: " + err.message);
  }
});

$("complete").addEventListener("click", async () => {
  if (!tripId) return;
  try {
    const t = await postJSON(`/v1/trips/${tripId}/complete`);
    $("state").textContent = t.state;
    $("complete").disabled = true;
  } catch (err) {
    logEvent("Complete failed: " + err.message);
  }
});

$("driverId").addEventListener("change", (e) => {
  driverId = e.target.value.trim() || randomId("driver");
  $("driverId").value = driverId;
  connect();
});

// --- log ---
function logEvent(msg) {
  const e = document.createElement("div");
  e.className = "entry";
  e.innerHTML = `<time>${new Date().toLocaleTimeString()}</time>${msg}`;
  $("log").appendChild(e);
}

refreshPos();
connect();
