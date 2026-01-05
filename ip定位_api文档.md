
# 完全免费无需注册的IP定位API接口文档

## 1. API接口列表
| 序号 | API名称       | 地址                                      | 调用示例                          | 返回字段示例 |
|------|---------------|-------------------------------------------|-----------------------------------|--------------|
| 1    | IP-API.com    | `http://ip-api.com/json/{IP地址}`          | `http://ip-api.com/json/8.8.8.8`   | `status`, `country`, `city`, `lat`, `lon` |
| 2    | ipapi.co      | `https://ipapi.co/{IP地址}/json/`          | `https://ipapi.co/8.8.8.8/json/`   | `ip`, `city`, `region`, `country`, `latitude`, `longitude` |
| 3    | ipapi.com     | `http://ip-api.com/json/{IP地址}?fields=...`| `http://ip-api.com/json/8.8.8.8?fields=status,message,country,countryCode,region,regionName,city,zip,lat,lon,timezone,isp,org,as,query` | `status`, `country`, `city`, `lat`, `lon`, `isp`, `org` |
| 4    | freegeoip.app | `https://freegeoip.app/json/{IP地址}`      | `https://freegeoip.app/json/8.8.8.8`| `ip`, `country_code`, `country_name`, `city`, `latitude`, `longitude` |
| 5    | ipinfo.io     | `https://ipinfo.io/{IP地址}/json`          | `https://ipinfo.io/8.8.8.8/json`   | `ip`, `hostname`, `city`, `region`, `country`, `loc`, `org` |
| 6    | geojs.io      | `https://get.geojs.io/v1/ip/geo/{IP地址}.json` | `https://get.geojs.io/v1/ip/geo/8.8.8.8.json` | `organization_name`, `accuracy`, `asn`, `timezone`, `longitude`, `country_code3` |
| 7    | ipstack.com   | `http://api.ipstack.com/{IP地址}?access_key=YOUR_ACCESS_KEY` | `http://api.ipstack.com/8.8.8.8?access_key=YOUR_ACCESS_KEY` | `ip`, `country_name`, `region_name`, `city`, `latitude`, `longitude` |
| 8    | ipdata.co     | `https://api.ipdata.co/{IP地址}`          | `https://api.ipdata.co/8.8.8.8`   | `ip`, `city`, `region`, `country_name`, `latitude`, `longitude` |
| 9    | ipgeolocation.io | `https://api.ipgeolocation.io/ipgeo?apiKey=YOUR_API_KEY&ip={IP地址}` | `https://api.ipgeolocation.io/ipgeo?apiKey=YOUR_API_KEY&ip=8.8.8.8` | `ip`, `city`, `region_name`, `country_name`, `latitude`, `longitude` |
| 10   | ipwhois.io    | `https://ipwhois.io/{IP地址}`             | `https://ipwhois.io/8.8.8.8`      | `ip`, `city`, `region`, `country`, `latitude`, `longitude` |

## 2. 数据说明
- 所有API均为完全免费，无需注册
- 返回字段包含IP地址、城市、经纬度等关键信息
- 部分API需替换`{IP地址}`为实际查询IP
- 部分API需替换`YOUR_ACCESS_KEY`或`YOUR_API_KEY`为个人密钥（非免费API）

## 3. 使用示例
```bash
# 查询IP地址8.8.8.8的地理位置
curl http://ip-api.com/json/8.8.8.8
```

## 4. 注意事项
- 请勿频繁请求，以免触发IP封禁
- 返回结果格式可能因API更新而变化
- 部分API需处理JSON解析
