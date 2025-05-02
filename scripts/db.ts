import fs from 'fs';
import path from 'path';
import {envVars} from './loadEnv';
import Database from 'better-sqlite3';


const db = new Database(path.resolve(__dirname, envVars.DB_PATH));
const schema = fs.readFileSync(path.resolve(__dirname, envVars.SCHEMA_PATH), 'utf-8');
db.exec(schema);

export { db }
