import { useEffect } from 'react';
import { Form, Input, InputNumber, Typography } from 'antd';
import type { Node } from '@xyflow/react';
import type { NodeType } from '../api/types';

const { Text } = Typography;

interface Props {
  node: Node | null;
  onChange: (id: string, data: Record<string, unknown>) => void;
}

export default function NodeConfigPanel({ node, onChange }: Props) {
  const [form] = Form.useForm<Record<string, unknown>>();

  useEffect(() => {
    if (node) {
      form.setFieldsValue(node.data as Record<string, never>);
    } else {
      form.resetFields();
    }
  }, [node, form]);

  if (!node) {
    return <Text type="secondary">Select a node to configure it.</Text>;
  }

  const type = node.type as NodeType;

  const handleChange = () => {
    const values = form.getFieldsValue() as Record<string, unknown>;
    onChange(node.id, values);
  };

  return (
    <Form
      form={form}
      layout="vertical"
      onValuesChange={handleChange}
      size="small"
    >
      {type === 'mqtt_source' && (
        <>
          <Form.Item
            label="Broker"
            name="broker"
            rules={[{ required: true, message: 'Broker is required' }]}
          >
            <Input placeholder="tcp://localhost:1883" />
          </Form.Item>
          <Form.Item
            label="Topic"
            name="topic"
            rules={[{ required: true, message: 'Topic is required' }]}
          >
            <Input placeholder="sensors/#" />
          </Form.Item>
          <Form.Item label="Username" name="username">
            <Input placeholder="(optional)" />
          </Form.Item>
          <Form.Item label="Password" name="password">
            <Input.Password placeholder="(optional)" />
          </Form.Item>
        </>
      )}

      {type === 'threshold' && (
        <>
          <Form.Item
            label="Min"
            name="min"
            rules={[
              {
                validator: async (_, value: unknown) => {
                  const max = form.getFieldValue('max') as number | undefined;
                  if (
                    value !== undefined &&
                    max !== undefined &&
                    (value as number) >= max
                  ) {
                    throw new Error('Min must be less than max');
                  }
                },
              },
            ]}
          >
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            label="Max"
            name="max"
            rules={[
              {
                validator: async (_, value: unknown) => {
                  const min = form.getFieldValue('min') as number | undefined;
                  if (
                    value !== undefined &&
                    min !== undefined &&
                    (value as number) <= min
                  ) {
                    throw new Error('Max must be greater than min');
                  }
                },
              },
            ]}
          >
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
        </>
      )}

      {type === 'rate_of_change' && (
        <>
          <Form.Item
            label="Max per second"
            name="max_per_second"
            rules={[
              { required: true },
              {
                validator: async (_, value: unknown) => {
                  if (typeof value === 'number' && value <= 0) {
                    throw new Error('Must be > 0');
                  }
                },
              },
            ]}
          >
            <InputNumber min={0.001} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            label="Window size"
            name="window_size"
            rules={[
              { required: true },
              {
                validator: async (_, value: unknown) => {
                  if (typeof value === 'number' && value < 2) {
                    throw new Error('Must be ≥ 2');
                  }
                },
              },
            ]}
          >
            <InputNumber min={2} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </>
      )}

      {type === 'debug_sink' && (
        <Text type="secondary">No configuration.</Text>
      )}
    </Form>
  );
}
