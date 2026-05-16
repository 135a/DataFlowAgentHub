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
    return <p style={{ color: "#999", fontSize: 13 }}>无可图表化的数据</p>;
  }

  const hasNumData = numKeys.length > 0;
  if (!hasNumData) {
    return <p style={{ color: "#999", fontSize: 13 }}>结果集中未检测到数值列</p>;
  }

  const containerStyle: CSSProperties = {
    width: "100%",
    height: 320,
    marginTop: 12,
  };

  if (renderError) {
    return (
      <div style={{ ...containerStyle, display: "flex", alignItems: "center", justifyContent: "center", background: "#fef3f2", borderRadius: 8, border: "1px solid #fecaca" }}>
        <p style={{ color: "#b91c1c", fontSize: 13 }}>图表渲染失败，请切换到表格视图</p>
      </div>
    );
  }

  try {
    return (
      <div>
        <div style={{ display: "flex", gap: 8, marginBottom: 8, marginTop: 8 }}>
          <button
            type="button"
            onClick={() => setChartType("bar")}
            style={{
              padding: "4px 12px",
              fontSize: 12,
              cursor: "pointer",
              fontWeight: chartType === "bar" ? "bold" : "normal",
              background: chartType === "bar" ? "#1a5fb4" : "#eef1f5",
              color: chartType === "bar" ? "#fff" : "#111",
              border: "none",
              borderRadius: 4,
            }}
          >
            柱状图
          </button>
          <button
            type="button"
            onClick={() => setChartType("line")}
            style={{
              padding: "4px 12px",
              fontSize: 12,
              cursor: "pointer",
              fontWeight: chartType === "line" ? "bold" : "normal",
              background: chartType === "line" ? "#1a5fb4" : "#eef1f5",
              color: chartType === "line" ? "#fff" : "#111",
              border: "none",
              borderRadius: 4,
            }}
          >
            折线图
          </button>
        </div>

        {truncated.length < rows.length && (
          <p style={{ fontSize: 11, color: "#999", margin: "0 0 4px" }}>
            仅展示前 {maxPoints} 条数据的图表
          </p>
        )}

        <div style={containerStyle}>
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
      <div style={{ ...containerStyle, display: "flex", alignItems: "center", justifyContent: "center", background: "#fef3f2", borderRadius: 8, border: "1px solid #fecaca" }}>
        <p style={{ color: "#b91c1c", fontSize: 13 }}>图表渲染失败，请切换到表格视图</p>
      </div>
    );
  }
}
