import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Form, Input, Button, Switch, Select, Card, Divider, Row, Col, message } from 'antd';
import { ArrowLeftOutlined, SaveOutlined } from '@ant-design/icons';
import { firewallApi, sitesApi } from '../services/api';

const WAFSettings: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [siteName, setSiteName] = useState('');

  useEffect(() => {
    if (id) {
      fetchData(id);
    }
  }, [id]);

  const fetchData = async (siteId: string) => {
    setLoading(true);
    try {
      // 1. Get Site Info for name
      const siteRes = await sitesApi.getSite(siteId);
      if (siteRes.code === 200) {
        setSiteName(siteRes.data.name);
      }

      // 2. Get WAF Config
      const wafRes = await firewallApi.getWafConfig(siteId);
      if (wafRes.code === 200) {
        const config = wafRes.data;
        // Map backend data to form
        form.setFieldsValue({
          enabled: config.enabled,
          custom_block_page: config.custom_block_page,
          rate_limit_count: config.rate_limit_count,
          rate_limit_window: config.rate_limit_window,
          blocked_countries: config.blocked_countries?.map((c: any) => c.country_code) || [],
          whitelist_ips: config.ip_whitelist?.map((i: any) => i.ip_address) || [],
          blacklist_ips: config.ip_blacklist?.map((i: any) => i.ip_address) || [],
        });
      }
    } catch (error) {
      console.error('Failed to fetch WAF settings:', error);
      message.error('获取WAF配置失败');
    } finally {
      setLoading(false);
    }
  };

  const onFinish = async (values: any) => {
    if (!id) return;
    setLoading(true);
    try {
      const res = await firewallApi.updateWafConfig(id, values);
      if (res.code === 200) {
        message.success('WAF配置保存成功');
      } else {
        message.error('保存失败: ' + res.message);
      }
    } catch (error) {
      console.error('Submit error:', error);
      message.error('保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '24px' }}>
      <div style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/sites')}>
          返回站点列表
        </Button>
      </div>
      
      <Card title={`WAF 防火墙配置 - ${siteName}`} loading={loading}>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            enabled: true,
            rate_limit_count: 100,
            rate_limit_window: 5,
          }}
        >
          <Form.Item name="enabled" label="启用防火墙" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Divider orientation="left">频率限制</Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="rate_limit_count" label="限制请求数" help="周期内允许的最大请求次数">
                <Input type="number" suffix="次" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="rate_limit_window" label="时间窗口" help="统计周期（分钟）">
                <Input type="number" suffix="分钟" />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left">访问控制</Divider>
          <Form.Item name="blocked_countries" label="禁止访问的国家/地区">
            <Select mode="tags" placeholder="输入国家代码，如 CN, US" tokenSeparators={[',', ' ']} />
          </Form.Item>

          <Form.Item name="whitelist_ips" label="IP 白名单">
            <Select mode="tags" placeholder="输入IP地址" tokenSeparators={[',', '\n']} />
          </Form.Item>

          <Form.Item name="blacklist_ips" label="IP 黑名单">
            <Select mode="tags" placeholder="输入IP地址" tokenSeparators={[',', '\n']} />
          </Form.Item>

          <Divider orientation="left">拦截页面</Divider>
          <Form.Item name="custom_block_page" label="自定义拦截页面HTML">
            <Input.TextArea rows={6} placeholder="<html><body><h1>Access Denied</h1></body></html>" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={loading}>
              保存配置
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default WAFSettings;
