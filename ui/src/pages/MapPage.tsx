import React, { useEffect, useMemo, useState } from 'react';
import { MapContainer, TileLayer, Marker, Polyline, Popup, useMap } from 'react-leaflet';
import type { LatLngExpression } from 'leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { Typography } from 'antd';
import { api, type Device } from '../api';
import { useSse } from '../useSse';

// Fix Leaflet default marker icon paths for Vite bundling.
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
});

const TRAIL_LEN = 30;

type Position = { lat: number; lng: number; time: string; payload: Record<string, unknown> };

function pickLatLng(payload: Record<string, unknown>): { lat: number; lng: number } | null {
  const lat = (payload.lat ?? payload.latitude) as unknown;
  const lng = (payload.lng ?? payload.lon ?? payload.longitude) as unknown;
  if (typeof lat === 'number' && typeof lng === 'number' && Number.isFinite(lat) && Number.isFinite(lng)) {
    return { lat, lng };
  }
  return null;
}

function AutoFit({ points }: { points: LatLngExpression[] }) {
  const map = useMap();
  useEffect(() => {
    if (points.length === 0) return;
    // Only fit once — do not refit on every update, or the user can't pan.
    map.fitBounds(points as [number, number][], { padding: [30, 30] });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [points.length > 0]);
  return null;
}

export default function MapPage() {
  const events = useSse('/api/v1/stream');
  const [devices, setDevices] = useState<Device[]>([]);

  useEffect(() => {
    api.listDevices().then(setDevices).catch(() => undefined);
  }, []);

  // Per-device trail of recent positions derived from SSE.
  const trailsByDevice = useMemo(() => {
    const out: Record<string, Position[]> = {};
    for (const e of events) {
      if (e.type !== 'telemetry') continue;
      const d = e.data as { device_id?: string; time?: string; payload?: Record<string, unknown> };
      if (!d.device_id || !d.time || !d.payload) continue;
      const p = pickLatLng(d.payload);
      if (!p) continue;
      const arr = out[d.device_id] ?? (out[d.device_id] = []);
      arr.push({ ...p, time: d.time, payload: d.payload });
      if (arr.length > TRAIL_LEN) arr.splice(0, arr.length - TRAIL_LEN);
    }
    return out;
  }, [events]);

  const deviceById = useMemo(() => {
    const m: Record<string, Device> = {};
    for (const d of devices) m[d.id] = d;
    return m;
  }, [devices]);

  const allPoints: LatLngExpression[] = useMemo(() => {
    const pts: LatLngExpression[] = [];
    for (const arr of Object.values(trailsByDevice)) {
      for (const p of arr) pts.push([p.lat, p.lng]);
    }
    return pts;
  }, [trailsByDevice]);

  const trailEntries = Object.entries(trailsByDevice);

  return (
    <div>
      <Typography.Title level={4} style={{ margin: 0, marginBottom: 8 }}>Live map</Typography.Title>
      <Typography.Text type="secondary">
        Devices reporting <code>lat</code>+<code>lng</code> (or <code>latitude</code>+<code>longitude</code>) in their payload appear here.
      </Typography.Text>
      <div style={{ height: 'calc(100vh - 200px)', marginTop: 8, border: '1px solid #eee' }}>
        <MapContainer center={[51.5074, -0.1278] as LatLngExpression} zoom={3} style={{ height: '100%', width: '100%' }}>
          <TileLayer
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          />
          <AutoFit points={allPoints} />
          {trailEntries.map(([deviceId, trail]) => {
            if (trail.length === 0) return null;
            const latest = trail[trail.length - 1];
            const dev = deviceById[deviceId];
            return (
              <React.Fragment key={deviceId}>
                {trail.length > 1 && (
                  <Polyline
                    positions={trail.map((p) => [p.lat, p.lng]) as LatLngExpression[]}
                    pathOptions={{ color: '#1677ff', weight: 3, opacity: 0.5 }}
                  />
                )}
                <Marker position={[latest.lat, latest.lng]}>
                  <Popup>
                    <div style={{ minWidth: 180 }}>
                      <div style={{ fontWeight: 600 }}>{dev?.name ?? deviceId.slice(0, 8)}</div>
                      <div style={{ fontSize: 11, color: '#888', marginBottom: 4 }}>
                        {new Date(latest.time).toLocaleTimeString()}
                      </div>
                      {Object.entries(latest.payload).map(([k, v]) => (
                        <div key={k} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                          <span style={{ color: '#666' }}>{k}</span>
                          <code>{typeof v === 'number' ? v.toFixed(5) : String(v)}</code>
                        </div>
                      ))}
                    </div>
                  </Popup>
                </Marker>
              </React.Fragment>
            );
          })}
        </MapContainer>
      </div>
    </div>
  );
}
