import { useEffect, useMemo, useState } from 'react';
import { Typography } from 'antd';
import DashboardGrid from '../dashboards/DashboardGrid';
import { api, type DashboardLayout, type Device } from '../api';

const DEMO_SENSOR_ID = '11111111-1111-1111-1111-111111111111';
const BUS_ID = '22222222-2222-2222-2222-222222222222';

export default function HomePage() {
  const [devices, setDevices] = useState<Device[]>([]);
  useEffect(() => { api.listDevices().then(setDevices); }, []);

  const layout: DashboardLayout = useMemo(() => {
    const hasSensor = devices.some((d) => d.id === DEMO_SENSOR_ID);
    const hasBus = devices.some((d) => d.id === BUS_ID);
    const widgets = [];
    if (hasSensor) {
      widgets.push({ id: 'v1', type: 'value' as const, config: { device_id: DEMO_SENSOR_ID, field: 'temperature', unit: '°C' }, x: 0, y: 0, w: 3, h: 2 });
      widgets.push({ id: 'v2', type: 'value' as const, config: { device_id: DEMO_SENSOR_ID, field: 'humidity', unit: '%' }, x: 3, y: 0, w: 3, h: 2 });
      widgets.push({ id: 'v3', type: 'value' as const, config: { device_id: DEMO_SENSOR_ID, field: 'battery', unit: '%' }, x: 6, y: 0, w: 3, h: 2 });
      widgets.push({ id: 'c1', type: 'chart' as const, config: { device_id: DEMO_SENSOR_ID }, x: 0, y: 2, w: 9, h: 5 });
    }
    if (hasBus) {
      widgets.push({ id: 'm1', type: 'map' as const, config: { device_id: BUS_ID }, x: 0, y: 7, w: 6, h: 5 });
    }
    widgets.push({ id: 'a1', type: 'alerts' as const, config: {}, x: 9, y: 0, w: 3, h: 7 });
    return { widgets };
  }, [devices]);

  return (
    <div>
      <Typography.Title level={4} style={{ margin: 0, marginBottom: 8 }}>Live overview</Typography.Title>
      <Typography.Text type="secondary">
        Example dashboard showcasing value tiles, a live chart, a map, and the alerts feed.
        Build your own on the <strong>Dashboards</strong> page.
      </Typography.Text>
      <div style={{ marginTop: 12 }}>
        <DashboardGrid value={layout} />
      </div>
    </div>
  );
}
