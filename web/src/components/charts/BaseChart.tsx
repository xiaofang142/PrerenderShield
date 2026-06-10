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

    const chart = echarts.init(chartRef.current)
    chartInstanceRef.current = chart

    try {
      chart.setOption(option, true)
      if (onChartReady) {
        onChartReady(chart)
      }
    } catch (error) {
      console.error('Failed to initialize chart:', error)
    }

    const handleResize = () => { chart.resize() }
    window.addEventListener('resize', handleResize)

    const observer = new ResizeObserver(() => chart.resize())
    if (chartRef.current) {
      observer.observe(chartRef.current)
    }

    return () => {
      window.removeEventListener('resize', handleResize)
      observer.disconnect()
      chart.dispose()
      chartInstanceRef.current = null
    }
  }, [option, onChartReady])

  return <div ref={chartRef} style={{ width: '100%', height: '100%', ...style }} />;
}

export default BaseChart