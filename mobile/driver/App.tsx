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
import MapView, { Marker, MarkerDragStartEndEvent } from "react-native-maps";
import * as Location from "expo-location";
import { DEFAULT_REGION } from "./src/config";
import { postJSON, randomId } from "./src/api";
import { useLive } from "./src/live";

type Coord = { latitude: number; longitude: number };

export default function App() {
  const [driverId, setDriverId] = useState(() => randomId("driver"));
  const [pos, setPos] = useState<Coord>({
    latitude: DEFAULT_REGION.latitude,
    longitude: DEFAULT_REGION.longitude,
  });
  const [online, setOnline] = useState(false);
  const [tripId, setTripId] = useState<string | null>(null);
  const [state, setState] = useState<string>("");
  const [lastPing, setLastPing] = useState<string>("—");
  const [log, setLog] = useState<string[]>([]);

  const addLog = useCallback((m: string) => {
    setLog((l) => [`${new Date().toLocaleTimeString()}  ${m}`, ...l].slice(0, 30));
  }, []);

  // Try to centre on the device's real location at startup.
  useEffect(() => {
    (async () => {
      try {
        const { status } = await Location.requestForegroundPermissionsAsync();
        if (status !== "granted") return;
        const loc = await Location.getCurrentPositionAsync({});
        setPos({ latitude: loc.coords.latitude, longitude: loc.coords.longitude });
      } catch {
        /* keep default */
      }
    })();
  }, []);

  const status = useLive(driverId, (n) => {
    addLog(n.message);
    if (!n.kind?.startsWith("trip:")) return;
    const s = n.kind.slice("trip:".length);
    if (s === "ASSIGNED") {
      setTripId(n.trip_id ?? null);
      setState("ASSIGNED");
    } else {
      setState(s);
    }
  });

  // Keep current values available to the ping interval without re-subscribing.
  const posRef = useRef(pos);
  posRef.current = pos;
  const onlineRef = useRef(online);
  onlineRef.current = online;
  const idRef = useRef(driverId);
  idRef.current = driverId;

  const ping = useCallback(async () => {
    try {
      const { latitude, longitude } = posRef.current;
      await postJSON(`/v1/drivers/${idRef.current}/location`, {
        lat: latitude,
        lng: longitude,
        available: onlineRef.current,
      });
      setLastPing(new Date().toLocaleTimeString());
    } catch (e: any) {
      addLog("Ping failed: " + e.message);
    }
  }, [addLog]);

  // Ping every 4s while online.
  useEffect(() => {
    if (!online) return;
    ping();
    const t = setInterval(ping, 4000);
    return () => clearInterval(t);
  }, [online, ping]);

  const toggleOnline = async () => {
    if (online) {
      setOnline(false);
      onlineRef.current = false;
      await ping(); // final ping marks unavailable
    } else {
      setOnline(true);
    }
  };

  const onDragEnd = (e: MarkerDragStartEndEvent) => {
    setPos(e.nativeEvent.coordinate);
    if (onlineRef.current) {
      posRef.current = e.nativeEvent.coordinate;
      ping();
    }
  };

  const startTrip = async () => {
    if (!tripId) return;
    try {
      const t = await postJSON(`/v1/trips/${tripId}/start`);
      setState(t.state);
    } catch (e: any) {
      addLog("Start failed: " + e.message);
    }
  };
  const completeTrip = async () => {
    if (!tripId) return;
    try {
      const t = await postJSON(`/v1/trips/${tripId}/complete`);
      setState(t.state);
    } catch (e: any) {
      addLog("Complete failed: " + e.message);
    }
  };

  return (
    <View style={styles.root}>
      <StatusBar style="light" />
      <MapView style={styles.map} initialRegion={DEFAULT_REGION}>
        <Marker coordinate={pos} draggable onDragEnd={onDragEnd} title="You" pinColor="#18c07a" />
      </MapView>

      <View style={styles.header}>
        <Text style={styles.title}>
          taxi · <Text style={styles.role}>Driver</Text>
        </Text>
        <View style={styles.spacer} />
        <TextInput style={styles.idInput} value={driverId} onChangeText={setDriverId} autoCapitalize="none" />
        <View style={[styles.pill, status === "connected" ? styles.on : styles.off]}>
          <Text style={styles.pillText}>{status === "connected" ? "live" : "offline"}</Text>
        </View>
      </View>

      <View style={styles.panel}>
        <View style={styles.rowBetween}>
          <Text style={styles.big}>{online ? "Online" : "Offline"}</Text>
          <Text style={styles.muted}>last ping {lastPing}</Text>
        </View>

        <TouchableOpacity style={[styles.btn, online ? styles.danger : styles.primary]} onPress={toggleOnline}>
          <Text style={styles.btnText}>{online ? "Go offline" : "Go online"}</Text>
        </TouchableOpacity>

        {tripId && (
          <View style={styles.tripBox}>
            <View style={styles.rowBetween}>
              <Text style={styles.muted}>trip</Text>
              <Text style={styles.state}>{state}</Text>
            </View>
            <View style={styles.rowBetween}>
              <TouchableOpacity
                style={[styles.btnSm, styles.primary, state !== "ASSIGNED" && styles.dim]}
                disabled={state !== "ASSIGNED"}
                onPress={startTrip}
              >
                <Text style={styles.btnText}>Start</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.btnSm, state !== "IN_PROGRESS" && styles.dim]}
                disabled={state !== "IN_PROGRESS"}
                onPress={completeTrip}
              >
                <Text style={styles.btnText}>Complete</Text>
              </TouchableOpacity>
            </View>
          </View>
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
  tripBox: { gap: 10, borderTopWidth: 1, borderTopColor: "#2a2f3a", paddingTop: 10 },
  btn: { borderRadius: 10, paddingVertical: 12, alignItems: "center" },
  btnSm: { flex: 1, borderRadius: 10, paddingVertical: 10, alignItems: "center", marginHorizontal: 3, backgroundColor: "#18c07a" },
  primary: { backgroundColor: "#2d7ff9" },
  danger: { backgroundColor: "#e5484d" },
  dim: { opacity: 0.4 },
  btnText: { color: "#fff", fontWeight: "700", fontSize: 15 },
  log: { maxHeight: 100, marginTop: 2 },
  logLine: { color: "#97a0b0", fontSize: 12, paddingVertical: 2 },
});
