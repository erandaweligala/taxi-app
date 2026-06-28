import { useCallback, useEffect, useRef, useState } from "react";
import {
  StyleSheet,
  Text,
  View,
  TouchableOpacity,
  ScrollView,
  TextInput,
} from "react-native";
import { StatusBar } from "expo-status-bar";
import MapView, { Marker, MapPressEvent, MarkerDragStartEndEvent } from "react-native-maps";
import { DEFAULT_REGION } from "./src/config";
import { postJSON, getJSON, randomId } from "./src/api";
import { useLive } from "./src/live";

type Coord = { latitude: number; longitude: number };

export default function App() {
  const [riderId, setRiderId] = useState(() => randomId("rider"));
  const [pickup, setPickup] = useState<Coord>({
    latitude: DEFAULT_REGION.latitude,
    longitude: DEFAULT_REGION.longitude,
  });
  const [surge, setSurge] = useState(1);
  const [supply, setSupply] = useState(0);
  const [driver, setDriver] = useState<Coord | null>(null);
  const [tripId, setTripId] = useState<string | null>(null);
  const [state, setState] = useState<string>("");
  const [log, setLog] = useState<string[]>([]);

  const addLog = useCallback((m: string) => {
    setLog((l) => [`${new Date().toLocaleTimeString()}  ${m}`, ...l].slice(0, 30));
  }, []);

  const status = useLive(riderId, (n) => {
    addLog(n.message);
    if (n.kind?.startsWith("trip:")) setState(n.kind.slice("trip:".length));
  });

  // Poll surge for the current pickup region.
  const pickupRef = useRef(pickup);
  pickupRef.current = pickup;
  useEffect(() => {
    const tick = async () => {
      try {
        const { latitude, longitude } = pickupRef.current;
        const s = await getJSON(`/v1/surge?lat=${latitude}&lng=${longitude}`);
        setSurge(s.surge);
        setSupply(s.supply);
      } catch {
        /* backend not reachable yet */
      }
    };
    tick();
    const id = setInterval(tick, 5000);
    return () => clearInterval(id);
  }, []);

  const requestRide = async () => {
    try {
      const r = await postJSON("/v1/rides", {
        rider_id: riderId,
        lat: pickup.latitude,
        lng: pickup.longitude,
      });
      setTripId(r.trip.id);
      setState(r.trip.state);
      setSurge(r.surge ?? 1);
      if (r.matched) {
        setDriver({ latitude: r.driver.lat, longitude: r.driver.lng });
        addLog(`Matched ${r.driver.driver_id} · ${r.driver.distance_km.toFixed(2)} km`);
      } else {
        setDriver(null);
        addLog(r.message ?? "No driver available");
      }
    } catch (e: any) {
      addLog("Request failed: " + e.message);
    }
  };

  const cancelRide = async () => {
    if (!tripId) return;
    try {
      await postJSON(`/v1/trips/${tripId}/cancel`);
    } catch (e: any) {
      addLog("Cancel failed: " + e.message);
    }
  };

  const onMapPress = (e: MapPressEvent) => setPickup(e.nativeEvent.coordinate);
  const onDragEnd = (e: MarkerDragStartEndEvent) => setPickup(e.nativeEvent.coordinate);

  const active = state && state !== "COMPLETED" && state !== "CANCELLED";

  return (
    <View style={styles.root}>
      <StatusBar style="light" />
      <MapView style={styles.map} initialRegion={DEFAULT_REGION} onPress={onMapPress}>
        <Marker coordinate={pickup} draggable onDragEnd={onDragEnd} title="Pickup" />
        {driver && <Marker coordinate={driver} pinColor="#2d7ff9" title="Your driver" />}
      </MapView>

      <View style={styles.header}>
        <Text style={styles.title}>
          taxi · <Text style={styles.role}>Rider</Text>
        </Text>
        <View style={styles.spacer} />
        <TextInput style={styles.idInput} value={riderId} onChangeText={setRiderId} autoCapitalize="none" />
        <View style={[styles.pill, status === "connected" ? styles.on : styles.off]}>
          <Text style={styles.pillText}>{status === "connected" ? "live" : "offline"}</Text>
        </View>
      </View>

      <View style={styles.panel}>
        <View style={styles.rowBetween}>
          <Text style={styles.big}>{surge.toFixed(2)}× surge</Text>
          <Text style={styles.muted}>{supply} drivers nearby</Text>
        </View>

        {active ? (
          <View style={styles.rowBetween}>
            <Text style={styles.state}>{state}</Text>
            <TouchableOpacity style={[styles.btn, styles.danger]} onPress={cancelRide}>
              <Text style={styles.btnText}>Cancel</Text>
            </TouchableOpacity>
          </View>
        ) : (
          <TouchableOpacity style={styles.btn} onPress={requestRide}>
            <Text style={styles.btnText}>Request ride</Text>
          </TouchableOpacity>
        )}

        <ScrollView style={styles.log}>
          {log.map((l, i) => (
            <Text key={i} style={styles.logLine}>
              {l}
            </Text>
          ))}
        </ScrollView>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#0f1115" },
  map: { position: "absolute", top: 0, left: 0, right: 0, bottom: 0 },
  header: {
    position: "absolute",
    top: 44,
    left: 12,
    right: 12,
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "rgba(24,27,34,0.92)",
    borderRadius: 12,
    paddingHorizontal: 12,
    paddingVertical: 8,
    gap: 8,
  },
  title: { color: "#e6e9ef", fontWeight: "700", fontSize: 15 },
  role: { color: "#18c07a" },
  spacer: { flex: 1 },
  idInput: {
    color: "#e6e9ef",
    backgroundColor: "#20242e",
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
    fontSize: 12,
    width: 110,
  },
  pill: { borderRadius: 999, paddingHorizontal: 8, paddingVertical: 3 },
  pillText: { fontSize: 11, fontWeight: "700", color: "#e6e9ef" },
  on: { backgroundColor: "rgba(24,192,122,0.3)" },
  off: { backgroundColor: "rgba(229,72,77,0.3)" },
  panel: {
    position: "absolute",
    left: 12,
    right: 12,
    bottom: 24,
    backgroundColor: "rgba(24,27,34,0.95)",
    borderRadius: 14,
    padding: 14,
    gap: 10,
  },
  rowBetween: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  big: { color: "#e6e9ef", fontSize: 20, fontWeight: "700" },
  muted: { color: "#97a0b0", fontSize: 13 },
  state: { color: "#18c07a", fontWeight: "700", fontSize: 16 },
  btn: { backgroundColor: "#18c07a", borderRadius: 10, paddingVertical: 12, alignItems: "center" },
  danger: { backgroundColor: "#e5484d", paddingHorizontal: 18, paddingVertical: 8 },
  btnText: { color: "#07150f", fontWeight: "700", fontSize: 15 },
  log: { maxHeight: 110, marginTop: 2 },
  logLine: { color: "#97a0b0", fontSize: 12, paddingVertical: 2 },
});
