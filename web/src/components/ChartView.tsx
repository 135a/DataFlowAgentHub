import { useState, useMemo, type CSSProperties } from "react";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import styles from "./ChartView.module.css";

interface Props {
  rows: Record<string, unknown>[];
  maxPoints?: number;
}

type ChartType = "bar" | "line";

const COLORS = [
  "#1a5fb4", "#26a269", "#c64600", "#865e3c", "#a51d2d",
  "#613583", "#1c71d8", "#33d17a", "#f5c211", "#e5a50a",
];

export function ChartView({ rows, maxPoints = 100 }: Props) {
  const [chartType, setChartType] = useState<ChartType>("bar");
  const [renderError, setRenderError] = useState(false);

  const truncated = rows.length > maxPoints ? rows.slice(0, maxPoints) : rows;

  const { xKey, numKeys, chartData, labelKey } = useMemo(() => {
    if (truncated.length === 0) return { xKey: null, numKeys: [] as string[], chartData: [], labelKey: "" };

    const keys = Object.keys(truncated[0]);

    // Separate numeric and non-numeric keys
    const numeric: string[] = [];
    let firstNonNum: string | null = null;
    for (const k of keys) {
      const v = truncated[0][k];
      if (typeof v === "number" && !firstNonNum) {
        // Check across all rows: is this actually the label column?
        // If first row has a number here, check if it's consistently numeric
        const allNumeric = truncated.every(r => typeof r[k] === "number");
        if (allNumeric) {
          numeric.push(k);
          continue;
        }
      }
      if (typeof v === "number") {
        numeric.push(k);
      } else if (!firstNonNum) {
        firstNonNum = k;
      }
    }

    // If no non-numeric column found, use the first numeric as label
    const label = firstNonNum || (numeric.length > 0 ? (numeric.shift() || keys[0]) : keys[0]);
    const remainingNum = firstNonNum ? numeric : numeric;

    const data = truncated.map((row) => {
      const item: Record<string, unknown> = {};
      item[label] = String(row[label] ?? "");
      for (const nk of remainingNum) {
        item[nk] = typeof row[nk] === "number" ? row[nk] : Number(row[nk]) || 0;
      }
      return item;
    });

    return { xKey: label, numKeys: remainingNum, chartData: data, labelKey: label };
  }, [truncated]);

  if (xKey === null || chartData.length === 0) {
    return <p className={styles.noDataText}>无可图表化的数据</p>;
  }

  const hasNumData = numKeys.length > 0;
  if (!hasNumData) {
    return <p className={styles.noNumericText}>结果集中未检测到数值列</p>;
  }

  const containerStyle: CSSProperties = {
    width: "100%",
    height: 320,
    marginTop: 12,
  };

  if (renderError) {
    return (
      <div className={`${styles.container} ${styles.errorContainer}`} style={containerStyle}>
        <p className={styles.errorText}>图表渲染失败，请切换到表格视图</p>
      </div>
    );
  }

  try {
    return (
      <div>
        <div className={styles.toolbar}>
          <button
            type="button"
            onClick={() => setChartType("bar")}
            className={`${styles.chartBtn} ${chartType === "bar" ? styles.chartBtnActive : styles.chartBtnInactive}`}
          >
            柱状图
          </button>
          <button
            type="button"
            onClick={() => setChartType("line")}
            className={`${styles.chartBtn} ${chartType === "line" ? styles.chartBtnActive : styles.chartBtnInactive}`}
          >
            折线图
          </button>
        </div>

        {truncated.length < rows.length && (
          <p className={styles.truncateNotice}>
            仅展示前 {maxPoints} 条数据的图表
          </p>
        )}

        <div className={styles.container} style={containerStyle}>
          <ResponsiveContainer width="100%" height="100%">
            {chartType === "bar" ? (
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
                <XAxis dataKey={xKey} tick={{ fontSize: 11 }} angle={-30} textAnchor="end" height={60} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Legend />
                {numKeys.map((key, i) => (
                  <Bar key={key} dataKey={key} fill={COLORS[i % COLORS.length]} />
                ))}
              </BarChart>
            ) : (
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
                <XAxis dataKey={xKey} tick={{ fontSize: 11 }} angle={-30} textAnchor="end" height={60} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Legend />
                {numKeys.map((key, i) => (
                  <Line key={key} type="monotone" dataKey={key} stroke={COLORS[i % COLORS.length]} strokeWidth={2} />
                ))}
              </LineChart>
            )}
          </ResponsiveContainer>
        </div>
      </div>
    );
  } catch {
    return (
      <div className={`${styles.container} ${styles.errorContainer}`} style={containerStyle}>
        <p className={styles.errorText}>图表渲染失败，请切换到表格视图</p>
      </div>
    );
  }
}
