import React from 'react'
import { Button, message, Dropdown } from 'antd'
import { DownloadOutlined, FileExcelOutlined, FileTextOutlined } from '@ant-design/icons'

interface ExportButtonProps {
  data: any[]
  columns: { title: string; dataIndex: string; key: string }[]
  filename?: string
}

const ExportButton: React.FC<ExportButtonProps> = ({ 
  data, 
  columns, 
  filename = 'export' 
}) => {
  // 导出为CSV
  const exportToCSV = () => {
    try {
      // 生成CSV内容
      const headers = columns.map(col => col.title).join(',')
      const rows = data.map(row => 
        columns.map(col => {
          let value = row[col.dataIndex]
          if (value === null || value === undefined) {
            value = ''
          }
          // 转义逗号和引号
          if (typeof value === 'string' && (value.includes(',') || value.includes('"') || value.includes('\n'))) {
            value = `"${value.replace(/"/g, '""')}"`
          }
          return value
        }).join(',')
      )
      
      const csvContent = [headers, ...rows].join('\n')
      
      // 创建Blob并下载
      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${filename}_${new Date().toISOString().split('T')[0]}.csv`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      
      message.success('CSV 导出成功')
    } catch (error) {
      message.error('导出失败')
      console.error('Export error:', error)
    }
  }

  // 导出为Excel (使用简单的Excel XML格式)
  const exportToExcel = () => {
    try {
      // 生成Excel XML内容
      let xml = '<?xml version="1.0" encoding="UTF-8"?>\n'
      xml += '<?mso-application progid="Excel.Sheet"?>\n'
      xml += '<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet"\n'
      xml += '  xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">\n'
      xml += '  <Styles>\n'
      xml += '    <Style ss:ID="header">\n'
      xml += '      <Font ss:Bold="1" />\n'
      xml += '    </Style>\n'
      xml += '  </Styles>\n'
      xml += '  <Worksheet ss:Name="Sheet1">\n'
      xml += '    <Table>\n'
      
      // 表头
      xml += '      <Row>\n'
      columns.forEach(col => {
        xml += `        <Cell ss:StyleID="header"><Data ss:Type="String">${col.title}</Data></Cell>\n`
      })
      xml += '      </Row>\n'
      
      // 数据行
      data.forEach(row => {
        xml += '      <Row>\n'
        columns.forEach(col => {
          const value = row[col.dataIndex] ?? ''
          const type = typeof value === 'number' ? 'Number' : 'String'
          xml += `        <Cell><Data ss:Type="${type}">${value}</Data></Cell>\n`
        })
        xml += '      </Row>\n'
      })
      
      xml += '    </Table>\n'
      xml += '  </Worksheet>\n'
      xml += '</Workbook>'
      
      // 创建Blob并下载
      const blob = new Blob([xml], { type: 'application/vnd.ms-excel' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${filename}_${new Date().toISOString().split('T')[0]}.xls`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      
      message.success('Excel 导出成功')
    } catch (error) {
      message.error('导出失败')
      console.error('Export error:', error)
    }
  }

  // 导出菜单项
  const menuItems = [
    {
      key: 'csv',
      icon: <FileTextOutlined />,
      label: '导出为 CSV',
      onClick: exportToCSV,
    },
    {
      key: 'excel',
      icon: <FileExcelOutlined />,
      label: '导出为 Excel',
      onClick: exportToExcel,
    },
  ]

  return (
    <Dropdown
      menu={{ items: menuItems }}
      trigger={['click']}
    >
      <Button icon={<DownloadOutlined />}>
        导出数据
      </Button>
    </Dropdown>
  )
}

export default ExportButton
