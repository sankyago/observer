import { useEffect, useMemo, useState } from 'react';
import { MapContainer, TileLayer, Marker, Polyline, Popup, useMap } from 'react-leaflet';
import type { LatLngExpression } from 'leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { Empty } from 'antd';
import { api } from '../api';
import { useSse } from '../useSse';

// Small, self-contained DivIcon — avoids broken <img> pins when Leaflet's
// default asset paths fail to resolve inside the Vite bundle.
function dotIcon(color: string, heading: number | null, speed: number | null): L.DivIcon {
  const arrow = heading !== null
    ? `<div style="position:absolute;top:-14px;left:50%;transform:translateX(-50%) rotate(${heading}deg);transform-origin:50% 34px;width:0;height:0;border-left:6px solid transparent;border-right:6px solid transparent;border-bottom:10px solid ${color};"></div>`
    : '';
  const badge = speed !== null && Number.isFinite(speed)
    ? `<div style="position:absolute;top:-8px;right:-8px;background:#000;color:#fff;font:600 10px system-ui;padding:1px 5px;border-radius:8px;">${speed.toFixed(0)}</div>`
    : '';
  return L.divIcon({
    html: `
      <div style="position:relative;">
        ${arrow}
        <div style="width:20px;height:20px;border-radius:50%;background:${color};border:3px solid white;box-shadow:0 1px 4px rgba(0,0,0,.35);"></div>
        ${badge}
      </div>`,
    className: 'observer-device-marker',
    iconSize: [20, 20],
    iconAnchor: [10, 10],
  });
}

function bearing(a: { lat: number; lng: number }, b: { lat: number; lng: number }): number {
  const toRad = (d: number) => (d * Math.PI) / 180;
  const toDeg = (r: number) => (r * 180) / Math.PI;
  const lat1 = toRad(a.lat), lat2 = toRad(b.lat);
  const dLng = toRad(b.lng - a.lng);
  const y = Math.sin(dLng) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLng);
  return (toDeg(Math.atan2(y, x)) + 360) % 360;
}

const TRAIL_LEN = 60;

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
  const ready = points.length > 0;
  useEffect(() => {
    if (!ready) return;
    map.fitBounds(points as [number, number][], { padding: [40, 40] });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready]);
  return null;
}

export default function DeviceMap({ deviceId }: { deviceId: string }) {
  const [trail, setTrail] = useState<Position[]>([]);
  const events = useSse('/api/v1/stream');

  // Seed with recent history (so we see a trail right away, not just live updates).
  useEffect(() => {
    let cancelled = false;
    api.recentTelemetry(deviceId, TRAIL_LEN).then((rows) => {
      if (cancelled) return;
      const pts: Position[] = rows
        .slice()
        .reverse()
        .map((r) => ({ ...pickLatLng(r.payload), time: r.time, payload: r.payload } as Position))
        .filter((p) => Number.isFinite(p.lat) && Number.isFinite(p.lng));
      setTrail(pts);
    });
    return () => { cancelled = true; };
  }, [deviceId]);

  // Append live SSE positions.
  useEffect(() => {
    if (events.length === 0) return;
    const last = events[events.length - 1];
    if (last.type !== 'telemetry') return;
    const d = last.data as { device_id?: string; time?: string; payload?: Record<string, unknown> };
    if (d.device_id !== deviceId || !d.time || !d.payload) return;
    const p = pickLatLng(d.payload);
    if (!p) return;
    setTrail((prev) => {
      const next = [...prev, { ...p, time: d.time!, payload: d.payload! }];
      return next.length > TRAIL_LEN ? next.slice(next.length - TRAIL_LEN) : next;
    });
  }, [events, deviceId]);

  const fitPoints: LatLngExpression[] = useMemo(
    () => trail.map((p) => [p.lat, p.lng] as LatLngExpression),
    [trail],
  );

  if (trail.length === 0) {
    return <Empty description="No lat/lng in this device's telemetry yet." />;
  }

  const latest = trail[trail.length - 1];
  const segments: Array<{ pts: LatLngExpression[]; opacity: number }> = [];
  for (let i = 1; i < trail.length; i++) {
    segments.push({
      pts: [
        [trail[i - 1].lat, trail[i - 1].lng],
        [trail[i].lat, trail[i].lng],
      ],
      opacity: 0.15 + 0.65 * (i / trail.length),
    });
  }

  return (
    <div style={{ width: '100%', height: '100%' }}>
      <MapContainer
        center={[latest.lat, latest.lng] as LatLngExpression}
        zoom={14}
        style={{ height: '100%', width: '100%' }}
      >
        <TileLayer
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        />
        <AutoFit points={fitPoints} />
        {segments.map((s, i) => (
          <Polyline
            key={i}
            positions={s.pts}
            pathOptions={{ color: '#1677ff', weight: 3, opacity: s.opacity }}
          />
        ))}
        <Marker
          position={[latest.lat, latest.lng]}
          icon={dotIcon(
            '#1677ff',
            trail.length >= 2 ? bearing(trail[trail.length - 2], latest) : null,
            typeof latest.payload.speed === 'number' ? (latest.payload.speed as number) : null,
          )}
        >
          <Popup>
            <div style={{ fontSize: 11, color: '#888', marginBottom: 4 }}>
              {new Date(latest.time).toLocaleString()}
            </div>
            {Object.entries(latest.payload).map(([k, v]) => (
              <div key={k} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, gap: 12 }}>
                <span style={{ color: '#666' }}>{k}</span>
                <code>{typeof v === 'number' ? v.toFixed(5) : String(v)}</code>
              </div>
            ))}
          </Popup>
        </Marker>
      </MapContainer>
    </div>
  );
}
