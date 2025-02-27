import { test } from "@playwright/test"
import { randomUUID } from "crypto"
import {
  createTemplate,
  createWorkspace,
  downloadCoderVersion,
  sshIntoWorkspace,
  startAgent,
} from "../helpers"

const clientVersion = "v0.14.0"

test("ssh with client " + clientVersion, async ({ page }) => {
  const token = randomUUID()
  const template = await createTemplate(page, {
    apply: [
      {
        complete: {
          resources: [
            {
              agents: [
                {
                  token,
                },
              ],
            },
          ],
        },
      },
    ],
  })
  const workspace = await createWorkspace(page, template)
  await startAgent(page, token)
  const binaryPath = await downloadCoderVersion(clientVersion)

  const client = await sshIntoWorkspace(page, workspace, binaryPath)
  await new Promise<void>((resolve, reject) => {
    // We just exec a command to be certain the agent is running!
    client.exec("exit 0", (err, stream) => {
      if (err) {
        return reject(err)
      }
      stream.on("exit", (code) => {
        if (code !== 0) {
          return reject(new Error(`Command exited with code ${code}`))
        }
        client.end()
        resolve()
      })
    })
  })
})
