/**
 * ECharts 按需引入：仅注册项目实际使用的图表/组件/渲染器，
 * 相比 `import * as echarts from 'echarts'` 全量导入可显著减小 bundle 体积。
 *
 * 当前使用的 series 类型：line / bar / pie / map（世界地图）/ gauge
 * 当前使用的组件：tooltip / legend / grid / title / visualMap / dataset
 */
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart, MapChart, GaugeChart } from 'echarts/charts'
import {
  TooltipComponent,
  LegendComponent,
  GridComponent,
  TitleComponent,
  VisualMapComponent,
  DatasetComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
  LineChart,
  BarChart,
  PieChart,
  MapChart,
  GaugeChart,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  TitleComponent,
  VisualMapComponent,
  DatasetComponent,
  CanvasRenderer,
])

// 完整版 EChartsOption 类型（含已注册图表的 series 类型提示）
export type { EChartsOption } from 'echarts'
export default echarts
