import DeviceChart from '../../devices/DeviceChart';

export default function ChartWidget({ deviceId }: { deviceId: string }) {
  return <div style={{ height: '100%' }}><DeviceChart deviceId={deviceId} /></div>;
}
