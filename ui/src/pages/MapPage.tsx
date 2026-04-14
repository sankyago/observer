import React, { useEffect, useMemo, useState } from 'react';
import { MapContainer, TileLayer, Marker, Polyline, Popup, useMap } from 'react-leaflet';
import type { LatLngExpression, Map as LeafletMap } from 'leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { Button, Checkbox, Space, Tag, Tooltip, Typography } from 'antd';
import { AimOutlined, CompassOutlined } from '@ant-design/icons';
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

const TRAIL_LEN = 40;
const DEVICE_COLORS = ['#1677ff', '#52c41a', '#fa541c', '#722ed1', '#eb2f96', '#faad14', '#13c2c2', '#f5222d'];

type Position = { lat: number; lng: number; time: string; payload: Record<string, unknown> };

function pickLatLng(payload: Record<string, unknown>): { lat: number; lng: number } | null {
  const lat = (payload.lat ?? payload.latitude) as unknown;
  const lng = (payload.lng ?? payload.lon ?? payload.longitude) as unknown;
  if (typeof lat === 'number' && typeof lng === 'number' && Number.isFinite(lat) && Number.isFinite(lng)) {
    return { lat, lng };
  }
  return null;
}

function bearing(a: Position, b: Position): number {
  const toRad = (d: number) => (d * Math.PI) / 180;
  const toDeg = (r: number) => (r * 180) / Math.PI;
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const dLng = toRad(b.lng - a.lng);
  const y = Math.sin(dLng) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(dLng);
  return (toDeg(Math.atan2(y, x)) + 360) % 360;
}

function hash(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) | 0;
  return Math.abs(h);
}

function deviceIcon(name: string, color: string, heading: number | null, speed: number | null): L.DivIcon {
  const hasHeading = heading !== null;
  const hasSpeed = speed !== null && Number.isFinite(speed);
  const arrow = hasHeading
    ? `<div style="position:absolute;top:-14px;left:50%;transform:translateX(-50%) rotate(${heading}deg);transform-origin:50% 34px;width:0;height:0;border-left:6px solid transparent;border-right:6px solid transparent;border-bottom:10px solid ${color};"></div>`
    : '';
  const speedBadge = hasSpeed
    ? `<div style="position:absolute;top:-8px;right:-8px;background:#000;color:#fff;font:600 10px system-ui;padding:1px 5px;border-radius:8px;white-space:nowrap;">${speed!.toFixed(0)}</div>`
    : '';
  const html = `
    <div style="position:relative;">
      ${arrow}
      <div style="
        width:20px;height:20px;border-radius:50%;background:${color};
        border:3px solid white;box-shadow:0 1px 4px rgba(0,0,0,.35);
      "></div>
      ${speedBadge}
      <div style="
        position:absolute;top:22px;left:50%;transform:translateX(-50%);
        background:white;color:#222;font:600 11px system-ui;
        padding:1px 6px;border-radius:4px;border:1px solid #ddd;white-space:nowrap;
      ">${name}</div>
    </div>
  `;
  return L.divIcon({ html, className: 'observer-device-marker', iconSize: [20, 20], iconAnchor: [10, 10] });
}

function AutoFit({ points }: { points: LatLngExpression[] }) {
  const map = useMap();
  const hasAny = points.length > 0;
  useEffect(() => {
    if (!hasAny) return;
    map.fitBounds(points as [number, number][], { padding: [40, 40] });
    // Fit only once on first points arrival — don't refight the user's pan/zoom.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasAny]);
  return null;
}

function MapController({ onReady }: { onReady: (m: LeafletMap) => void }) {
  const map = useMap();
  useEffect(() => { onReady(map); }, [map, onReady]);
  return null;
}

export default function MapPage() {
  const events = useSse('/api/v1/stream');
  const [devices, setDevices] = useState<Device[]>([]);
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [mapRef, setMapRef] = useState<LeafletMap | null>(null);

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

  const deviceColor = (id: string) => DEVICE_COLORS[hash(id) % DEVICE_COLORS.length];

  // Points for initial auto-fit (all visible devices' latest positions).
  const fitPoints: LatLngExpression[] = useMemo(() => {
    const pts: LatLngExpression[] = [];
    for (const [id, arr] of Object.entries(trailsByDevice)) {
      if (hidden.has(id) || arr.length === 0) continue;
      const latest = arr[arr.length - 1];
      pts.push([latest.lat, latest.lng]);
    }
    return pts;
  }, [trailsByDevice, hidden]);

  const trailEntries = Object.entries(trailsByDevice);
  const visibleEntries = trailEntries.filter(([id]) => !hidden.has(id));

  const fitAll = () => {
    if (!mapRef || fitPoints.length === 0) return;
    mapRef.fitBounds(fitPoints as [number, number][], { padding: [40, 40] });
  };

  const centerOn = (deviceId: string) => {
    if (!mapRef) return;
    const arr = trailsByDevice[deviceId];
    if (!arr || arr.length === 0) return;
    const latest = arr[arr.length - 1];
    mapRef.setView([latest.lat, latest.lng], Math.max(mapRef.getZoom(), 14), { animate: true });
  };

  return (
    <div>
      <Space style={{ marginBottom: 8, width: '100%', justifyContent: 'space-between' }}>
        <Typography.Title level={4} style={{ margin: 0 }}>Live map</Typography.Title>
        <Button icon={<AimOutlined />} onClick={fitAll} disabled={fitPoints.length === 0}>
          Fit to devices
        </Button>
      </Space>
      <Typography.Text type="secondary">
        Devices reporting <code>lat</code>+<code>lng</code> (or <code>latitude</code>+<code>longitude</code>) in their payload appear here.
        Markers show heading (from last two points) and speed (if <code>speed</code> in payload).
      </Typography.Text>

      <div style={{ display: 'grid', gridTemplateColumns: '240px 1fr', gap: 12, marginTop: 8, height: 'calc(100vh - 240px)' }}>
        <div style={{ border: '1px solid #eee', padding: 8, overflowY: 'auto' }}>
          <Typography.Text strong style={{ fontSize: 12 }}>Devices</Typography.Text>
          <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 4 }}>
            {trailEntries.length === 0 && (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>no positions yet</Typography.Text>
            )}
            {trailEntries.map(([id, arr]) => {
              const dev = deviceById[id];
              const color = deviceColor(id);
              const latest = arr[arr.length - 1];
              return (
                <div key={id} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Checkbox
                    checked={!hidden.has(id)}
                    onChange={(e) => {
                      const next = new Set(hidden);
                      if (e.target.checked) next.delete(id); else next.add(id);
                      setHidden(next);
                    }}
                  />
                  <span style={{ width: 10, height: 10, borderRadius: '50%', background: color, display: 'inline-block' }} />
                  <span
                    style={{ fontSize: 12, cursor: 'pointer', flex: 1 }}
                    onClick={() => centerOn(id)}
                    title="Click to center map"
                  >
                    {dev?.name ?? id.slice(0, 8)}
                  </span>
                  {typeof latest.payload.speed === 'number' && (
                    <Tooltip title="speed">
                      <Tag color="geekblue" style={{ margin: 0 }}>{(latest.payload.speed as number).toFixed(0)}</Tag>
                    </Tooltip>
                  )}
                </div>
              );
            })}
          </div>
        </div>

        <div style={{ border: '1px solid #eee' }}>
          <MapContainer center={[51.5074, -0.1278] as LatLngExpression} zoom={3} style={{ height: '100%', width: '100%' }}>
            <TileLayer
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
            />
            <MapController onReady={setMapRef} />
            <AutoFit points={fitPoints} />
            {visibleEntries.map(([deviceId, trail]) => {
              if (trail.length === 0) return null;
              const latest = trail[trail.length - 1];
              const prev = trail.length >= 2 ? trail[trail.length - 2] : null;
              const dev = deviceById[deviceId];
              const color = deviceColor(deviceId);
              const heading = prev ? bearing(prev, latest) : null;
              const speed = typeof latest.payload.speed === 'number' ? (latest.payload.speed as number) : null;

              // Render trail as short segments with fading opacity for "comet tail" effect.
              const segments: Array<{ pts: LatLngExpression[]; opacity: number }> = [];
              for (let i = 1; i < trail.length; i++) {
                const opacity = 0.15 + 0.65 * (i / trail.length);
                segments.push({
                  pts: [
                    [trail[i - 1].lat, trail[i - 1].lng],
                    [trail[i].lat, trail[i].lng],
                  ],
                  opacity,
                });
              }

              return (
                <React.Fragment key={deviceId}>
                  {segments.map((seg, i) => (
                    <Polyline
                      key={`seg-${deviceId}-${i}`}
                      positions={seg.pts}
                      pathOptions={{ color, weight: 3, opacity: seg.opacity }}
                    />
                  ))}
                  <Marker
                    position={[latest.lat, latest.lng]}
                    icon={deviceIcon(dev?.name ?? deviceId.slice(0, 6), color, heading, speed)}
                  >
                    <Popup>
                      <div style={{ minWidth: 200 }}>
                        <div style={{ fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ width: 10, height: 10, borderRadius: '50%', background: color, display: 'inline-block' }} />
                          {dev?.name ?? deviceId.slice(0, 8)}
                        </div>
                        <div style={{ fontSize: 11, color: '#888', marginBottom: 4 }}>
                          {new Date(latest.time).toLocaleString()}
                        </div>
                        {heading !== null && (
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12 }}>
                            <span style={{ color: '#666' }}><CompassOutlined /> heading</span>
                            <code>{heading.toFixed(0)}°</code>
                          </div>
                        )}
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
    </div>
  );
}
