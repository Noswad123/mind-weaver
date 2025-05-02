import dotenv from 'dotenv';
import { loadNeorgConfig } from './loadNeorgConfig';

dotenv.config();

interface EnvVars {
  SCHEMA_PATH: string;
  DB_PATH: string;
  NOTES_DIR: string;
  NEORG_CONFIG: string;
}

function validateEnvVar(key: keyof EnvVars): string {
  console.log(`Validating ${key}...`);
  if (!process.env[key]) {
    throw new Error(`${key} environment variable is not set`);
  }
  console.log(process.env[key]);
  return process.env[key]!;
}

const envVars: EnvVars = {
  SCHEMA_PATH: validateEnvVar('SCHEMA_PATH'),
  DB_PATH: validateEnvVar('DB_PATH'),
  NOTES_DIR: validateEnvVar('NOTES_DIR'),
  NEORG_CONFIG: validateEnvVar('NEORG_CONFIG'),
};

const workspaceMap: Record<string, string> = loadNeorgConfig(envVars.NEORG_CONFIG).workspaces;

export { envVars, workspaceMap };
