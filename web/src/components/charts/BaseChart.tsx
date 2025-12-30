import React, { useEffect, useRef } from 'react'
import * as echarts from 'echarts'

interface BaseChartProps {
  option: echarts.EChartsOption
  style?: React.CSSProperties
  onChartReady?: (chart: echarts.ECharts) => void
}

const BaseChart: React.FC<BaseChartProps> = ({ option, style, onChartReady }) => {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!chartRef.current) return

    // 初始化图表
    const chart = echarts.init(chartRef.current)
    chartInstanceRef.current = chart

    // 设置图表选项，添加错误处理
    try {
      chart.setOption(option)

      // 注册图表就绪回调
      if (onChartReady) {
        onChartReady(chart)
      }
    } catch (error) {
      console.error('Failed to initialize chart:', error)
    }

    // 响应窗口大小变化
    const handleResize = () => {
      chart.resize()
    }
    window.addEventListener('resize', handleResize)

    // 清理函数
    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartInstanceRef.current = null
    }
  }, [option, onChartReady])

  // 更新图表选项，添加错误处理
  useEffect(() => {
    if (chartInstanceRef.current) {
      try {
        chartInstanceRef.current.setOption(option, true)
      } catch (error) {
        console.error('Failed to update chart option:', error)
      }
    }
  }, [option])

  return <div ref={chartRef} style={{ width: '100%', height: '100%', ...style }} />;
}

export default BaseChart