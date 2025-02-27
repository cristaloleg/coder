import Box from "@mui/material/Box"
import { useQuery } from "@tanstack/react-query"
import { getHealth } from "api/api"
import { Loader } from "components/Loader/Loader"
import { useTab } from "hooks"
import { Helmet } from "react-helmet-async"
import { pageTitle } from "utils/page"
import { colors } from "theme/colors"
import CheckCircleOutlined from "@mui/icons-material/CheckCircleOutlined"
import ErrorOutline from "@mui/icons-material/ErrorOutline"
import { SyntaxHighlighter } from "components/SyntaxHighlighter/SyntaxHighlighter"
import { Stack } from "components/Stack/Stack"
import {
  FullWidthPageHeader,
  PageHeaderTitle,
  PageHeaderSubtitle,
} from "components/PageHeader/FullWidthPageHeader"
import { Stats, StatsItem } from "components/Stats/Stats"
import { makeStyles } from "@mui/styles"
import { createDayString } from "utils/createDayString"

const sections = {
  derp: "DERP",
  access_url: "Access URL",
  websocket: "Websocket",
  database: "Database",
} as const

export default function HealthPage() {
  const tab = useTab("tab", "derp")
  const { data: healthStatus } = useQuery({
    queryKey: ["health"],
    queryFn: () => getHealth(),
    refetchInterval: 10_000,
  })

  return (
    <>
      <Helmet>
        <title>{pageTitle("Health")}</title>
      </Helmet>

      {healthStatus ? (
        <HealthPageView healthStatus={healthStatus.data} tab={tab} />
      ) : (
        <Loader />
      )}
    </>
  )
}

export function HealthPageView({
  healthStatus,
  tab,
}: {
  healthStatus: Awaited<ReturnType<typeof getHealth>>["data"]
  tab: ReturnType<typeof useTab>
}) {
  const styles = useStyles()

  return (
    <Box
      sx={{
        height: "calc(100vh - 62px - 36px)",
        overflow: "hidden",
        // Remove padding added from dashboard layout (.siteContent)
        marginBottom: "-48px",
      }}
    >
      <FullWidthPageHeader sticky={false}>
        <Stack direction="row" spacing={2} alignItems="center">
          {healthStatus.healthy ? (
            <CheckCircleOutlined
              sx={{
                width: 32,
                height: 32,
                color: (theme) => theme.palette.success.light,
              }}
            />
          ) : (
            <ErrorOutline
              sx={{
                width: 32,
                height: 32,
                color: (theme) => theme.palette.error.main,
              }}
            />
          )}

          <div>
            <PageHeaderTitle>
              {healthStatus.healthy ? "Healthy" : "Unhealthy"}
            </PageHeaderTitle>
            <PageHeaderSubtitle>
              {healthStatus.healthy
                ? "All systems operational"
                : "Some issues have been detected"}
            </PageHeaderSubtitle>
          </div>
        </Stack>

        <Stats aria-label="Deployment details" className={styles.stats}>
          <StatsItem
            className={styles.statsItem}
            label="Last check"
            value={createDayString(healthStatus.time)}
          />
          <StatsItem
            className={styles.statsItem}
            label="Coder version"
            value={healthStatus.coder_version}
          />
        </Stats>
      </FullWidthPageHeader>
      <Box
        sx={{
          display: "flex",
          alignItems: "start",
          height: "100%",
        }}
      >
        <Box
          sx={{
            width: (theme) => theme.spacing(32),
            flexShrink: 0,
            borderRight: (theme) => `1px solid ${theme.palette.divider}`,
            height: "100%",
          }}
        >
          <Box
            sx={{
              fontSize: 10,
              textTransform: "uppercase",
              fontWeight: 500,
              color: (theme) => theme.palette.text.secondary,
              padding: (theme) => theme.spacing(1.5, 3),
              letterSpacing: "0.5px",
            }}
          >
            Health
          </Box>
          <Box component="nav">
            {Object.keys(sections)
              .sort()
              .map((key) => {
                const label = sections[key as keyof typeof sections]
                const isActive = tab.value === key
                const isHealthy =
                  healthStatus[key as keyof typeof sections].healthy

                return (
                  <Box
                    component="button"
                    key={key}
                    onClick={() => {
                      tab.set(key)
                    }}
                    sx={{
                      background: isActive ? colors.gray[13] : "none",
                      border: "none",
                      fontSize: 14,
                      width: "100%",
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                      textAlign: "left",
                      height: 36,
                      padding: (theme) => theme.spacing(0, 3),
                      cursor: "pointer",
                      pointerEvents: isActive ? "none" : "auto",
                      color: (theme) =>
                        isActive
                          ? theme.palette.text.primary
                          : theme.palette.text.secondary,
                      "&:hover": {
                        background: (theme) => theme.palette.action.hover,
                        color: (theme) => theme.palette.text.primary,
                      },
                    }}
                  >
                    {isHealthy ? (
                      <CheckCircleOutlined
                        sx={{
                          width: 16,
                          height: 16,
                          color: (theme) => theme.palette.success.light,
                        }}
                      />
                    ) : (
                      <ErrorOutline
                        sx={{
                          width: 16,
                          height: 16,
                          color: (theme) => theme.palette.error.main,
                        }}
                      />
                    )}
                    {label}
                  </Box>
                )
              })}
          </Box>
        </Box>
        {/* 62px - navbar and 36px - the bottom bar */}
        <Box sx={{ height: "100%", overflowY: "auto", width: "100%" }}>
          <SyntaxHighlighter
            language="json"
            editorProps={{ height: "100%" }}
            value={JSON.stringify(
              healthStatus[tab.value as keyof typeof healthStatus],
              null,
              2,
            )}
          />
        </Box>
      </Box>
    </Box>
  )
}

const useStyles = makeStyles((theme) => ({
  stats: {
    padding: 0,
    border: 0,
    gap: theme.spacing(6),
    rowGap: theme.spacing(3),
    flex: 1,

    [theme.breakpoints.down("md")]: {
      display: "flex",
      flexDirection: "column",
      alignItems: "flex-start",
      gap: theme.spacing(1),
    },
  },

  statsItem: {
    flexDirection: "column",
    gap: 0,
    padding: 0,

    "& > span:first-of-type": {
      fontSize: 12,
      fontWeight: 500,
    },
  },
}))
