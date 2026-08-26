import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Form, Input, Button, Switch, Select, Card, Divider, Row, Col, message } from 'antd';
import { ArrowLeftOutlined, SaveOutlined } from '@ant-design/icons';
import { firewallApi, sitesApi } from '../services/api';
import { useTranslation } from 'react-i18next';

const WAFSettings: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [siteName, setSiteName] = useState('');
  // 竞态防护：路由参数快速变化时，旧请求的响应不再写入 state
  const requestVersionRef = useRef(0);

  useEffect(() => {
    if (id) {
      fetchData(id);
    }
  }, [id]);

  const fetchData = async (siteId: string) => {
    const version = ++requestVersionRef.current;
    setLoading(true);
    try {
      // 1. Get Site Info for name
      const siteRes = await sitesApi.getSite(siteId);
      if (version !== requestVersionRef.current) return;
      if (siteRes.code === 200) {
        setSiteName(siteRes.data.name);
      }

      // 2. Get WAF Config
      const wafRes = await firewallApi.getWafConfig(siteId);
      if (version !== requestVersionRef.current) return;
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
      message.error(t('wafSettings.fetchConfigFailed'));
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
        message.success(t('wafSettings.saveSuccess'));
      } else {
        message.error(t('wafSettings.saveFailedWithReason', { reason: res.message }));
      }
    } catch (error) {
      console.error('Submit error:', error);
      message.error(t('wafSettings.saveFailed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '24px' }}>
      <div style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/sites')}>
          {t('wafSettings.backToSites')}
        </Button>
      </div>

      <Card title={t('wafSettings.title', { siteName })} loading={loading}>
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
          <Form.Item name="enabled" label={t('wafSettings.enableFirewall')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Divider orientation="left">{t('wafSettings.rateLimitSection')}</Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="rate_limit_count" label={t('wafSettings.requestCountLabel')} help={t('wafSettings.requestCountHelp')}>
                <Input type="number" suffix={t('wafSettings.countUnit')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="rate_limit_window" label={t('wafSettings.windowLabel')} help={t('wafSettings.windowHelp')}>
                <Input type="number" suffix={t('wafSettings.minutesUnit')} />
              </Form.Item>
            </Col>
          </Row>

          <Divider orientation="left">{t('wafSettings.accessControlSection')}</Divider>
          <Form.Item name="blocked_countries" label={t('wafSettings.blockedCountries')}>
            <Select mode="tags" placeholder={t('wafSettings.countryPlaceholder')} tokenSeparators={[',', ' ']} />
          </Form.Item>

          <Form.Item name="whitelist_ips" label={t('wafSettings.whitelistIps')}>
            <Select mode="tags" placeholder={t('wafSettings.ipPlaceholder')} tokenSeparators={[',', '\n']} />
          </Form.Item>

          <Form.Item name="blacklist_ips" label={t('wafSettings.blacklistIps')}>
            <Select mode="tags" placeholder={t('wafSettings.ipPlaceholder')} tokenSeparators={[',', '\n']} />
          </Form.Item>

          <Divider orientation="left">{t('wafSettings.blockPageSection')}</Divider>
          <Form.Item name="custom_block_page" label={t('wafSettings.customBlockPage')}>
            <Input.TextArea rows={6} placeholder="<html><body><h1>Access Denied</h1></body></html>" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={loading}>
              {t('wafSettings.saveConfig')}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default WAFSettings;
