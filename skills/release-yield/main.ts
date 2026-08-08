import { defineSkill } from "@operatorstack/yield"
import { runReleaseYield } from "./src/workflow.ts"

defineSkill(runReleaseYield)
