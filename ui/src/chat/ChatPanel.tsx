import { useEffect, useRef, useState } from 'react';
import { Button, Input, Space, Spin, Typography } from 'antd';
import { SendOutlined, RobotOutlined, UserOutlined } from '@ant-design/icons';
import { api, type ChatMessage } from '../api';

export default function ChatPanel() {
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: 'assistant',
      content:
        "Hi! I'm the Observer assistant (stub). Ask me about devices, flows, or dashboards — I'll be wired to a real model soon.",
    },
  ]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages, sending]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;
    const next: ChatMessage[] = [...messages, { role: 'user', content: text }];
    setMessages(next);
    setInput('');
    setSending(true);
    try {
      const reply = await api.chat(next);
      setMessages([...next, { role: 'assistant', content: reply.content }]);
    } catch (e) {
      setMessages([...next, { role: 'assistant', content: `error: ${String(e)}` }]);
    } finally {
      setSending(false);
    }
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '12px 16px', borderBottom: '1px solid #f0f0f0' }}>
        <Space>
          <RobotOutlined style={{ color: '#722ed1' }} />
          <Typography.Text strong>Observer AI</Typography.Text>
        </Space>
      </div>

      <div ref={listRef} style={{ flex: 1, overflowY: 'auto', padding: 12, background: '#fafafa' }}>
        {messages.map((m, i) => (
          <div
            key={i}
            style={{
              display: 'flex',
              gap: 8,
              marginBottom: 12,
              flexDirection: m.role === 'user' ? 'row-reverse' : 'row',
            }}
          >
            <div style={{ fontSize: 18, color: m.role === 'user' ? '#1677ff' : '#722ed1', flexShrink: 0 }}>
              {m.role === 'user' ? <UserOutlined /> : <RobotOutlined />}
            </div>
            <div
              style={{
                background: m.role === 'user' ? '#1677ff' : 'white',
                color: m.role === 'user' ? 'white' : '#333',
                padding: '8px 12px',
                borderRadius: 8,
                maxWidth: '85%',
                whiteSpace: 'pre-wrap',
                fontSize: 13,
                boxShadow: '0 1px 2px rgba(0,0,0,0.06)',
              }}
            >
              {m.content}
            </div>
          </div>
        ))}
        {sending && (
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', color: '#888', fontSize: 12 }}>
            <Spin size="small" />
            <span>thinking…</span>
          </div>
        )}
      </div>

      <div style={{ padding: 12, borderTop: '1px solid #f0f0f0', background: 'white' }}>
        <Input.TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
          placeholder="Ask about devices, flows, dashboards…"
          autoSize={{ minRows: 2, maxRows: 6 }}
          disabled={sending}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          onClick={send}
          disabled={!input.trim() || sending}
          style={{ marginTop: 8, width: '100%' }}
        >
          Send
        </Button>
      </div>
    </div>
  );
}
