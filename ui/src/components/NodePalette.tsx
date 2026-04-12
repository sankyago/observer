import { Card } from 'antd';
import {
  ApiOutlined,
  AlertOutlined,
  LineChartOutlined,
  BugOutlined,
} from '@ant-design/icons';
import type { NodeType } from '../api/types';

interface PaletteItem {
  type: NodeType;
  label: string;
  icon: React.ReactNode;
}

const ITEMS: PaletteItem[] = [
  { type: 'mqtt_source', label: 'MQTT Source', icon: <ApiOutlined /> },
  { type: 'threshold', label: 'Threshold', icon: <AlertOutlined /> },
  { type: 'rate_of_change', label: 'Rate of Change', icon: <LineChartOutlined /> },
  { type: 'debug_sink', label: 'Debug Sink', icon: <BugOutlined /> },
];

export default function NodePalette() {
  const onDragStart = (e: React.DragEvent, type: NodeType) => {
    e.dataTransfer.setData('application/reactflow', type);
    e.dataTransfer.effectAllowed = 'move';
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {ITEMS.map((item) => (
        <Card
          key={item.type}
          size="small"
          draggable
          onDragStart={(e) => onDragStart(e, item.type)}
          style={{ cursor: 'grab', userSelect: 'none' }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {item.icon}
            <span style={{ fontSize: 13 }}>{item.label}</span>
          </div>
        </Card>
      ))}
    </div>
  );
}
