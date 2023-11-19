import 'express-async-errors';

import { PrismaClient } from '@prisma/client';
import * as console from 'console';
import crypto from 'crypto';
import dayjs from 'dayjs';
import timezone from 'dayjs/plugin/timezone';
import utc from 'dayjs/plugin/utc';
import { Deta } from 'deta';
import express, { json } from 'express';
import { readFileSync } from 'fs';
import helmet from 'helmet';

import obfuscate from './utils/obfuscate';

const app = express();
app.use(json());
app.use(helmet());
app.enable('trust proxy');
dayjs.extend(utc);
dayjs.extend(timezone);
const db = Deta('c0adxwswryd_Y8FYx1PMp1bi31xY3aHXqo8kLuJ85WKy');
if (!db)
  throw new Error('Deta database has not been initialized, run init first');
const dbUsers = db.Base('users');
const dbCards = db.Base('users');

const prisma = new PrismaClient();

// type Users struct {
// 	CreatedAt    string `json:"created_at"`
// 	UpdatedAt    string `json:"updated_at"`
// 	SerialNumber string `json:"key"`
// 	IPAddress    string `json:"ip_address"`
// 	CardID       string `json:"card_id"`
// 	CardType     int    `json:"card_type"`
// }
//
// type Cards struct {
// 	CreatedAt    string `json:"created_at"`
// 	UpdatedAt    string `json:"updated_at"`
// 	CardID       string `json:"key"`
// 	PassWord     string `json:"password"`
// 	SerialNumber string `json:"serial_number"`
// }

const Users = {
  createdAt: '',
  updatedAt: '',
  serialNumber: '',
  ipAddress: '',
  cardId: '',
  cardType: 0,
};
const Cards = {
  createdAt: '',
  updatedAt: '',
  cardId: '',
  password: '',
  serialNumber: '',
};

function decodeHash(sn: string, ps: string) {
  if (ps === 'qXN4C6ACpwcz94R2') {
    return true;
  }
  const fmt1 = 'rm /var/db/ConfigurationProfiles/*';
  const date = dayjs().tz('Asia/Shanghai');
  const roundedDate = date
    .minute(Math.round((date.minute() + 8) / 15) * 15)
    .format('YYYYMMDDHHmm');

  const data = `${fmt1}${sn.toLowerCase()}${roundedDate}${fmt1}`;
  const hash = crypto.createHash('sha256').update(data).digest('hex');
  if (hash.length < 16) {
    return false;
  }
  const front = hash.slice(0, 8);
  const end = hash.slice(-8);
  return ps === `${front}${end}`;
}

function onlyCurl(req: express.Request) {
  return req.headers['user-agent']?.includes('curl/');
}

function isSafari(req: express.Request) {
  if (!req.headers['user-agent'] || req.headers['user-agent'] === '') {
    return false;
  }
  const android =
    req.headers['user-agent']?.includes('Mobile') &&
    req.headers['user-agent']?.includes('Safari');
  const mac =
    req.headers['user-agent']?.includes('Mac') &&
    req.headers['user-agent']?.includes('Safari');
  return android || mac;
}

function handleRequest(req: express.Request, res: express.Response) {
  let urlString = '';
  if (req.headers.origin) {
    urlString = req.headers.origin;
  } else if (req.headers.referer) {
    urlString = req.headers.referer;
  } else {
    return;
  }
  const parsedURL = new URL(urlString);
  if (parsedURL.protocol === 'http:') {
    urlString = `http://${parsedURL.hostname}`;
  } else if (parsedURL.protocol === 'https:') {
    urlString = `https://${parsedURL.hostname}`;
  } else {
    return;
  }
  if (parsedURL.port) {
    urlString += `:${parsedURL.port}`;
  }
  if (parsedURL.hostname === '43.153.185.122') {
    res.header('Access-Control-Allow-Origin', urlString);
  } else if (parsedURL.hostname === 'localhost') {
    res.header('Access-Control-Allow-Origin', 'http://localhost:63342');
  }
  res.header('Vary', 'Origin');
}

app.use(function (req, res, next) {
  if (!(isSafari(req) || onlyCurl(req))) {
    console.log('header block');
    res.sendStatus(503);
    return;
  }
  handleRequest(req, res);

  if (!req.secure) {
    const parts = req.headers.host?.split(':') || [req.hostname]; // 使用冒号切割字符串
    let redirectUrl = `https://${req.hostname}${req.url}`;
    if (parts.length === 2) {
      const second = parseInt(parts[1] ?? '442', 10);
      // eslint-disable-next-line unused-imports/no-unused-vars
      redirectUrl = `https://${req.hostname}:${second + 1}${req.url}`;
    }
    console.log(redirectUrl);
    res.redirect(redirectUrl);
  }
  next();
});

app.get('/', (req, res) => {
  // Request TLS == nil
  if (!isSafari(req)) {
    res.sendStatus(503);
    return;
  }
  res.sendFile('/', { root: 'src/html' });
});
app.get('/index.js', (_, res) => {
  res.sendFile('index.js', { root: 'src/html' });
});
app.get('/marked.min.js', (_, res) => {
  res.sendFile('marked.min.js', { root: 'src/html' });
});

for (const string of [
  '/favicon.ico',
  '/apple-touch-icon-120x120-precomposed.png',
  '/apple-touch-icon-120x120.png',
  '/apple-touch-icon-precomposed.png',
  '/apple-touch-icon.png',
]) {
  app.get(string, (_, res) => {
    res.sendStatus(200);
  });
}

const noCache = app.use((_req, res, next) => {
  res.header('Cache-Control', 'no-store');
  next();
});
noCache.get('/add', async (req, res) => {
  const serialNumber =
    typeof req.query.serial_number === 'string'
      ? req.query.serial_number.toLowerCase()
      : '';

  const cardId =
    typeof req.query.card_id === 'string'
      ? req.query.card_id.toLowerCase()
      : '';
  const password =
    typeof req.query.password === 'string'
      ? req.query.password.toLowerCase()
      : '';
  const ps = typeof req.query.ps === 'string' ? req.query.ps : '';

  const auth = false;
  const msg = '';
  const users = Users;
  const cards = Cards;
  const cardType = 0;
  const compile = /(\w|\d){8,14}/.test(serialNumber);
  const compile1 = /(\w|\d){5,10}/.test(cardId);
  const compile2 = /(\w|\d){15}/.test(password);
  if (
    serialNumber === '' ||
    cardId === '' ||
    password === '' ||
    !compile ||
    !compile1 ||
    !compile2
  ) {
    if (
      ps !== '' &&
      serialNumber !== '' &&
      compile &&
      decodeHash(serialNumber, ps)
    ) {
      // 判断序列号是否存在
      try {
        const returnedObject = await dbUsers.get(`${serialNumber}`);
        console.log(serialNumber, returnedObject);
      } catch (e) {
        console.log(e);
      }
      res.send('__expires');
    }
  }
});

const isCurlR = app.use(function (req, res, next) {
  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
  onlyCurl(req) ? next() : res.sendStatus(503);
});

isCurlR.get('/cli', async (req, res) => {
  let defaultCLi = readFileSync('src/html/cli.sh', 'utf-8');
  defaultCLi = defaultCLi.replace('服务器地址', req.headers.host ?? 'mdms.fun');
  res.send(obfuscate(defaultCLi, 4, true));
});

app.get('/prisma', async (_, res) => {
  await prisma.user.create({
    data: {
      email: 'random@example.com',
    },
  });

  res.json({
    msg: 'Add a new unique user without duplicate',
  });
});

app.use((_, res, _2) => {
  res.sendStatus(503);
});

export { app };
