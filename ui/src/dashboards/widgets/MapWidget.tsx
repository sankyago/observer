import DeviceMap from '../../devices/DeviceMap';

export default function MapWidget({ deviceId }: { deviceId: string }) {
  return <div style={{ height: '100%' }}><DeviceMap deviceId={deviceId} /></div>;
}
