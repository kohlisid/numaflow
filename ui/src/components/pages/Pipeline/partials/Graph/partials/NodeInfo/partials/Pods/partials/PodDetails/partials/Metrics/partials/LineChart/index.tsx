import { useCallback, useContext, useEffect, useState } from "react";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Text,
} from "recharts";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Dropdown from "../common/Dropdown";
import FiltersDropdown from "../common/FiltersDropdown";
import EmptyChart from "../EmptyChart";
import { useMetricsFetch } from "../../../../../../../../../../../../../../../utils/fetchWrappers/metricsFetch";
import TimeSelector from "../common/TimeRange";
import { AppContext } from "../../../../../../../../../../../../../../../App";
import { AppContextProps } from "../../../../../../../../../../../../../../../types/declarations/app";

interface TooltipProps {
  payload?: any[];
  label?: string;
  active?: boolean;
}

function CustomTooltip({ payload, label, active, patternName }: TooltipProps & { patternName: string }) {
  if (active && payload && payload.length) {
    const maxWidth =
      Math.max(...payload.map((entry) => entry?.name?.length)) * 9.5;
    return (
      <Box
        sx={{
          backgroundColor: "#fff",
          padding: "1rem",
          border: "0.1rem solid #ccc",
          borderRadius: "1rem",
        }}
      >
        <Box>{label}</Box>
        {payload.map((entry: any, index: any) => {
          const formattedValue = getDefaultFormatter(entry?.value, patternName);
          return(
          <Box key={`item-${index}`} sx={{ display: "flex" }}>
            <Box
              sx={{
                width: `${maxWidth / 9}rem`,
                display: "inline-block",
                paddingRight: "1rem",
                color: entry?.color,
              }}
            >
              {entry?.name}:
            </Box>
            <Box sx={{ color: entry?.color }}>{formattedValue}</Box>
          </Box>
          );
        })}
      </Box>
    );
  }
  return null;
}

const getYAxisLabel = (unit: string) => {
  if (unit !== "") {
    return unit;
  }
  return "";
};

const getDefaultFormatter = (value: number, patternName: string) => {
  const formatValue = (value: number, suffix: string) => {
    const formattedValue = parseFloat(value?.toFixed(2));
    return formattedValue % 1 === 0
      ? `${Math.floor(formattedValue)}${suffix}`
      : `${formattedValue}${suffix}`;
  };
  switch(patternName){
    case "mono_vertex_histogram":
      if (value === 0){
        return "0";
      } else if (value < 1000) {
        return formatValue(value," μs");
      } else if (value < 1000000) {
        return formatValue(value / 1000, " ms");
      } else {
        return formatValue(value / 1000000, " s");
      }
    case "container_cpu_memory_utilization":
    case "pod_cpu_memory_utilization":
      if (value === 0){
        return "0";
      } else if (value < 1000) {
        return formatValue(value," %");
      } else if (value < 1000000) {
        return formatValue(value / 1000, "k %");
      } else {
        return formatValue(value / 1000000, "M %");
      }
    default:
      if (value === 0) {
        return "0";
      } else if (value < 1000) {
        return formatValue(value,"");
      } else if (value < 1000000) {
        return formatValue(value / 1000, " k");
      } else {
        return formatValue(value / 1000000, " M");
      }
  }
};

const getTickFormatter = (unit: string, patternName: string) => {
  const formatValue = (value: number) => {
    const formattedValue = parseFloat(value?.toFixed(2)); // Format to 2 decimal places
    return formattedValue % 1 === 0
      ? Math.floor(formattedValue)
      : formattedValue; // Remove trailing .0
  };
  return (value: number) => {
    switch (unit) {
      case "s":
        return `${formatValue(value / 1000000)}`;
      case "ms":
        return `${formatValue(value / 1000)}`;
      default:
        return getDefaultFormatter(value, patternName);
    }
  };
};

interface LineChartComponentProps {
  namespaceId: string;
  pipelineId: string;
  type: string;
  metric: any;
  vertexId?: string;
  selectedPodName?: string;
  fromModal?: boolean;
}

// TODO have a check for metricReq against metric object to ensure required fields are passed
const LineChartComponent = ({
  namespaceId,
  pipelineId,
  type,
  metric,
  vertexId,
  selectedPodName,
  fromModal,
}: LineChartComponentProps) => {
  const { addError } = useContext<AppContextProps>(AppContext);
  const [transformedData, setTransformedData] = useState<any[]>([]);
  const [chartLabels, setChartLabels] = useState<any[]>([]);
  const [metricsReq, setMetricsReq] = useState<any>({
    metric_name: metric?.metric_name,
    pattern_name: metric?.pattern_name,
  });
  const [paramsList, setParamsList] = useState<any[]>([]);
  // store all filters for each selected dimension
  const [filtersList, setFiltersList] = useState<any[]>([]);
  const [filters, setFilters] = useState<any>({});
  const [previousDimension, setPreviousDimension] = useState<string>(
    metricsReq?.dimension
  );

  const getRandomColor = useCallback((index: number) => {
    const hue = (index * 137.508) % 360;
    return `hsl(${hue}, 50%, 50%)`;
  }, []);

  // required filters
  const getFilterValue = useCallback(
    (filterName: string) => {
      switch (filterName) {
        case "namespace":
          return namespaceId;
        case "mvtx_name":
        case "pipeline":
          return pipelineId;
        case "vertex":
          return vertexId;
        case "pod":
          // based on pattern names, update filter based on pod value or multiple pods based on regex
          if (metric?.pattern_name === "pod_cpu_memory_utilization"){
            switch(type){
              case "monoVertex":
                return `${pipelineId}-.*`;
              default:
                return `${pipelineId}-${vertexId}-.*`;
            }
          }
          else {
            return selectedPodName;
          }
        default:
          return "";
      }
    },
    [namespaceId, pipelineId]
  );

  const updateFilterList = useCallback(
    (dimensionVal: string) => {
      const newFilters =
        metric?.dimensions
          ?.find((dimension: any) => dimension?.name === dimensionVal)
          ?.filters?.map((param: any) => ({
            name: param?.Name,
            required: param?.Required,
          })) || [];
      setFiltersList(newFilters);
    },
    [metric, setFiltersList]
  );

  const updateFilters = useCallback(() => {
    const newFilters: any = {};
    filtersList?.forEach((filterElement: any) => {
      if (filterElement?.name && filterElement?.required) {
        newFilters[filterElement.name] = getFilterValue(filterElement.name);
      }
    });
    setFilters(newFilters);
  }, [filtersList, getFilterValue, setFilters]);

  //update filters only when dimension changes in metricsReq
  useEffect(() => {
    if (metricsReq?.dimension !== previousDimension) {
      updateFilterList(metricsReq.dimension);
      setPreviousDimension(metricsReq?.dimension);
    }
  }, [metricsReq, updateFilterList]);

  useEffect(() => {
    if (filtersList?.length) updateFilters();
  }, [filtersList]);

  const updateParams = useCallback(() => {
    const initParams = [{ name: "dimension", required: "true" }];
    // taking dimension[0] as all will have same params
    const newParams =
      metric?.dimensions?.[0]?.params?.map((param: any) => ({
        name: param?.Name,
        required: param?.Required,
      })) || [];

    setParamsList([...initParams, ...newParams]);
  }, [metric, setParamsList]);

  // update params once initially
  useEffect(() => {
    updateParams();
  }, [updateParams]);

  const { chartData, error, isLoading } = useMetricsFetch({
    metricReq: metricsReq,
    filters,
  });

  useEffect(() => {
    if (error) {
      addError(error?.toString());
    }
  }, [error, addError]);

  const groupByLabel = useCallback((dimension: string, patternName: string) => {
    switch(patternName){
      case "pod_cpu_memory_utilization":
        return ["pod"];
      case "container_cpu_memory_utilization":
        return ["container"];
      case "mono_vertex_gauge":
      case "vertex_gauge":
        return dimension === "pod" ? ["pod", "period"] : ["period"];
    }
    switch (dimension) {
      case "mono-vertex":
        return ["mvtx_name"];
      default:
        return [dimension];
    }
  }, []);

  const updateChartData = useCallback(() => {
    if (chartData) {
      const labels: any[] = [];
      const transformedData: any[] = [];
      const label = groupByLabel(
        metricsReq?.dimension,
        metricsReq?.pattern_name
      );
      chartData?.forEach((item) => {
        let labelVal = "";
        label?.forEach((eachLabel: string) => {
          if (item?.metric?.[eachLabel] !== undefined) {
            labelVal += (labelVal ? "-" : "") + item.metric[eachLabel];
          }
        });

        // Remove initial hyphen if labelVal is not empty
        if (labelVal.startsWith("-") && labelVal.length > 1) {
          labelVal = labelVal.substring(1);
        }

        labels.push(labelVal);
        item?.values?.forEach(([timestamp, value]: [number, string]) => {
          const date = new Date(timestamp * 1000);
          const hours = date.getHours().toString().padStart(2, "0");
          const minutes = date.getMinutes().toString().padStart(2, "0");
          const formattedTime = `${hours}:${minutes}`;
          const ele = transformedData?.find(
            (data) => data?.time === formattedTime
          );
          if (!ele) {
            const dataObject: Record<string, any> = { time: formattedTime };
            dataObject[labelVal] = parseFloat(value);
            transformedData.push(dataObject);
          } else {
            ele[labelVal] = parseFloat(value);
          }
        });
      });
      transformedData.sort((a, b) => {
        const [hoursA, minutesA] = a.time.split(":").map(Number);
        const [hoursB, minutesB] = b.time.split(":").map(Number);
        return hoursA * 60 + minutesA - (hoursB * 60 + minutesB);
      });
      setChartLabels(labels);
      setTransformedData(transformedData);
    }
  }, [chartData, metricsReq, groupByLabel]);

  useEffect(() => {
    if (chartData) updateChartData();
  }, [chartData, updateChartData]);

  if (paramsList?.length === 0) return <></>;

  const hasTimeParams = paramsList?.some((param) =>
    ["start_time", "end_time"].includes(param?.name)
  );

  const getMetricsModalDesc = () => {
    return `This chart represents the above metric at a ${metricsReq?.dimension} level over the selected time period.`;
  };

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-around",
          mt: "1rem",
          mb: "2rem",
        }}
      >
        {paramsList
          ?.filter(
            (param) => !["start_time", "end_time"]?.includes(param?.name)
          )
          ?.map((param: any) => {
            return (
              <Box
                display={fromModal ? "none" : "flex"}
                key={`line-chart-${param?.name}`}
                sx={{ minWidth: 120, fontSize: "2rem" }}
              >
                <Dropdown
                  metric={metric}
                  type={type}
                  field={param?.name}
                  setMetricReq={setMetricsReq}
                />
              </Box>
            );
          })}
        {fromModal && (
          <Box
            sx={{ display: "flex", alignItems: "center", fontSize: "1.4rem" }}
          >
            {getMetricsModalDesc()}
          </Box>
        )}
        {hasTimeParams && (
          <Box key="line-chart-preset">
            <TimeSelector setMetricReq={setMetricsReq} />
          </Box>
        )}
      </Box>

      {filtersList?.filter((filterEle: any) => !filterEle?.required)?.length >
        0 && (
        <Box
          sx={{
            display: fromModal ? "none" : "flex",
            alignItems: "center",
            justifyContent: "space-around",
            mt: "1rem",
            mb: "2rem",
            px: "6rem",
          }}
        >
          <Box sx={{ mr: "1rem" }}>Filters</Box>
          <FiltersDropdown
            items={filtersList?.filter(
              (filterEle: any) => !filterEle?.required
            )}
            namespaceId={namespaceId}
            pipelineId={pipelineId}
            type={type}
            vertexId={vertexId}
            setFilters={setFilters}
            selectedPodName={selectedPodName}
            metric={metric}
          />
        </Box>
      )}

      {isLoading && (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            height: "100%",
          }}
        >
          <CircularProgress />
        </Box>
      )}

      {!isLoading && error && <EmptyChart message={error?.toString()} />}

      {!isLoading && !error && transformedData?.length > 0 && (
        <ResponsiveContainer width="100%" height={400}>
          <LineChart
            data={transformedData}
            margin={{
              top: 5,
              right: 30,
              left: 30,
              bottom: 5,
            }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="time" padding={{ left: 30, right: 30 }}></XAxis>
            <YAxis
              label={
                <Text
                  x={-160}
                  y={15}
                  dy={5}
                  transform="rotate(-90)"
                  fontSize={14}
                  textAnchor="middle"
                >
                  {getYAxisLabel(metric?.unit)}
                </Text>
              }
              tickFormatter={getTickFormatter(
                metric?.unit,
                metric?.pattern_name
              )}
            />
            <CartesianGrid stroke="#f5f5f5"></CartesianGrid>

            {chartLabels?.map((value, index) => (
              <Line
                key={`${value}-line-chart`}
                type="monotone"
                dataKey={`${value}`}
                stroke={getRandomColor(index)}
                activeDot={{ r: 8 }}
              />
            ))}

            <Tooltip content={<CustomTooltip patternName={metric?.pattern_name} />}/>
            <Legend />
          </LineChart>
        </ResponsiveContainer>
      )}

      {!isLoading && !error && transformedData?.length === 0 && <EmptyChart />}
    </Box>
  );
};

export default LineChartComponent;
