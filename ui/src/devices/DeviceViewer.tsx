import { useState } from 'react';
import { Segmented, Tooltip } from 'antd';
import { LineChartOutlined, EnvironmentOutlined, TableOutlined } from '@ant-design/icons';
import DeviceChart from './DeviceChart';
import DeviceMap from './DeviceMap';
import DeviceRaw from './DeviceRaw';

type View = 'chart' | 'map' | 'raw';

export default function DeviceViewer({ deviceId }: { deviceId: string }) {
  const [view, setView] = useState<View>('chart');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 12 }}>
      <Segmented
        value={view}
        onChange={(v) => setView(v as View)}
        options={[
          {
            value: 'chart',
            icon: (
              <Tooltip title="Chart">
                <LineChartOutlined />
              </Tooltip>
            ),
            label: 'Chart',
          },
          {
            value: 'map',
            icon: (
              <Tooltip title="Map">
                <EnvironmentOutlined />
              </Tooltip>
            ),
            label: 'Map',
          },
          {
            value: 'raw',
            icon: (
              <Tooltip title="Raw">
                <TableOutlined />
              </Tooltip>
            ),
            label: 'Raw',
          },
        ]}
      />
      <div style={{ flex: 1, minHeight: 0 }}>
        {view === 'chart' && <DeviceChart deviceId={deviceId} />}
        {view === 'map' && <DeviceMap deviceId={deviceId} />}
        {view === 'raw' && <DeviceRaw deviceId={deviceId} />}
      </div>
    </div>
  );
}
