// @ts-ignore — react-grid-layout typedefs use namespace exports; we use the CommonJS default
import GridLayout from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import { useMemo } from 'react';
import ChartWidget from './widgets/ChartWidget';
import MapWidget from './widgets/MapWidget';
import ValueWidget from './widgets/ValueWidget';
import AlertsWidget from './widgets/AlertsWidget';
import type { DashboardLayout, Widget } from '../api';

type LayoutItem = { i: string; x: number; y: number; w: number; h: number };

type Props = {
  value: DashboardLayout;
  onChange?: (next: DashboardLayout) => void;
  width?: number;
  cols?: number;
};

export default function DashboardGrid({ value, onChange, width = 1200, cols = 12 }: Props) {
  const editable = !!onChange;

  const layouts: LayoutItem[] = useMemo(
    () => value.widgets.map((w): LayoutItem => ({ i: w.id, x: w.x, y: w.y, w: w.w, h: w.h })),
    [value.widgets],
  );

  const applyLayout = (next: LayoutItem[]) => {
    if (!onChange) return;
    const byId = new Map(next.map((l) => [l.i, l]));
    onChange({
      widgets: value.widgets.map((w) => {
        const l = byId.get(w.id);
        if (!l) return w;
        return { ...w, x: l.x, y: l.y, w: l.w, h: l.h };
      }),
    });
  };

  const GL = GridLayout as unknown as React.ComponentType<{
    className?: string;
    layout: LayoutItem[];
    cols: number;
    rowHeight: number;
    width: number;
    onLayoutChange: (next: LayoutItem[]) => void;
    isDraggable: boolean;
    isResizable: boolean;
    compactType: string | null;
    draggableCancel?: string;
    children: React.ReactNode;
  }>;

  return (
    <GL
      className="layout"
      layout={layouts}
      cols={cols}
      rowHeight={50}
      width={width}
      onLayoutChange={applyLayout}
      isDraggable={editable}
      isResizable={editable}
      compactType="vertical"
      draggableCancel=".no-drag"
    >
      {value.widgets.map((w) => (
        <div key={w.id} style={{ background: 'white', borderRadius: 4, overflow: 'hidden' }}>
          <div className="no-drag" style={{ height: '100%', width: '100%' }}>
            <WidgetRenderer widget={w} />
          </div>
        </div>
      ))}
    </GL>
  );
}

function WidgetRenderer({ widget }: { widget: Widget }) {
  const c = widget.config as Record<string, string | undefined>;
  switch (widget.type) {
    case 'chart':
      return c.device_id ? <ChartWidget deviceId={c.device_id} /> : <Empty />;
    case 'map':
      return c.device_id ? <MapWidget deviceId={c.device_id} /> : <Empty />;
    case 'value':
      return c.device_id && c.field ? (
        <ValueWidget deviceId={c.device_id} field={c.field} unit={c.unit} />
      ) : <Empty />;
    case 'alerts':
      return <AlertsWidget />;
    default:
      return <Empty />;
  }
}

function Empty() {
  return <div style={{ padding: 12, color: '#aaa', fontSize: 12 }}>configure this widget</div>;
}
