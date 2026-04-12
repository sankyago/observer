import type { NodeTypes } from '@xyflow/react';
import MQTTSourceNode from './MQTTSourceNode';
import ThresholdNode from './ThresholdNode';
import RateOfChangeNode from './RateOfChangeNode';
import DebugSinkNode from './DebugSinkNode';

export const nodeTypes: NodeTypes = {
  mqtt_source: MQTTSourceNode,
  threshold: ThresholdNode,
  rate_of_change: RateOfChangeNode,
  debug_sink: DebugSinkNode,
};

export { MQTTSourceNode, ThresholdNode, RateOfChangeNode, DebugSinkNode };
