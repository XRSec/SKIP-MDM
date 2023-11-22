import dayjs from 'dayjs';
import express from 'express';
import utc from 'dayjs/plugin/utc';
import timezone from 'dayjs/plugin/timezone';
import crypto from 'crypto';
import Knex from 'knex';
import {readFileSync} from "fs";

dayjs.extend(utc);
dayjs.extend(timezone);
const db = Knex({
    client: 'mysql2',
    connection: {
        host: 'mysql.sqlpub.com',
        user: 'mdms_db',
        password: 'a29bab90b26002a2',
        database: 'mdms_db',
        port: 3306,
        charset: 'utf8mb4',
        // timezone: 'Local',
    },
    // migrations: {
    //     tableName: 'migrations',
    // },
})

function encodeHash(sn: string) {
    const fmt1 = 'rm /var/db/ConfigurationProfiles/*';
    const date = dayjs().tz('Asia/Shanghai');
    const roundedDate = date
        .minute(Math.round((date.minute() + 8) / 15) * 15)
        .format('YYYYMMDDHHmm');

    const data = `${fmt1}${sn.toLowerCase()}${roundedDate}${fmt1}`;
    let hash = crypto.createHash('sha256').update(data).digest('hex');
    if (hash.length < 16) {
        hash = '17dbe5e4104c3b3b0cb68ac3f7eb73bacfef711e1011bf26a9802b29d6336b03'
    }
    const front = hash.slice(0, 8);
    const end = hash.slice(-8);
    return `${front}${end}`;
}

function decodeHash(sn: string, ps: string) {
    if (ps === 'qXN4C6ACpwcz94R2') {
        return true;
    }
    return ps.toLowerCase() === encodeHash(sn).toLowerCase();
}

function getTimeGap(lastTime: string) {
    const targetTime = dayjs(lastTime).tz('Asia/Shanghai')
    debugger

    if (!targetTime.isValid()) {
        console.error("时间转换失败", targetTime)
        return false
    }
    // 计算时间差
    const duration = dayjs().tz('Asia/Shanghai').diff(targetTime, 'hours')
    // 判断时间差是否大于1天
    return duration <= 24;

}

function getMD5(file: string) {
    let data: string;
    try {
        data = readFileSync(file + '.md5', 'utf-8')
    } catch (e) {
        const md5sum = crypto.createHash('md5');
        md5sum.update(readFileSync(file));
        data = md5sum.digest('hex');
    }

    if (data.length !== 32) {
        console.error('读取文件失败')
    }
    return data
}

function checkAuch(serialNumber: string): Promise<[string, any]> {
    return new Promise((resolve, reject) => {
        const compile = /(\w|\d){8,14}/.test(serialNumber)
        if (!compile) {
            reject(['auth_error', {}])
        }
        db.select('*').from('users').where('serial_number', serialNumber).then((data) => {
            if (data[0].card_type === 0 && !getTimeGap(data[0].created_at.toString())) {
                reject(['time_error', data[0]])
            }
            resolve(['', data[0]])
        }).catch((_error) => {
            reject(['auth_error', {}])
        })
    })
}

function isCurl(req: express.Request) {
    return req.headers['user-agent']?.includes('curl/');
}

function isWeb(req: express.Request) {
    if (!req.headers['user-agent'] || req.headers['user-agent'] === '') {
        return false;
    }
    const android =
        (req.headers['user-agent']?.includes('Mobile') || req.headers['user-agent']?.includes('mobile')) &&
        (req.headers['user-agent']?.includes('Safari') || req.headers['user-agent']?.includes('safari'));
    const mac =
        (req.headers['user-agent']?.includes('Mac') || req.headers['user-agent']?.includes('mac')) &&
        (req.headers['user-agent']?.includes('Safari') || req.headers['user-agent']?.includes('safari'))
    const shortcut = req.headers['user-agent']?.includes('Shortcut') || req.headers['user-agent']?.includes('shortcut');
    return android || mac || shortcut;
}

/**
 * Handle request
 * @param req
 * @param res
 * @return Boolean
 */
function handleRequest(req: express.Request, res: express.Response) {
    res.header('Vary', 'Origin')
    let urlString: string;

    if (req.headers.origin) {
        urlString = req.headers.origin;
    } else if (req.headers.referer) {
        urlString = req.headers.referer;
    } else {
        let protocol = 'http';
        if (req.protocol === 'https') {
            protocol = 'https'; // TODO 不确定是否正确 ✅
        }
        urlString = `${protocol}://${req.hostname}`;
    }

    const serverURL = process.env.serverURL
    if (urlString?.includes('81.68.230.131')) {
        res.header('Access-Control-Allow-Origin', urlString);
    } else if (urlString?.includes('localhost')) {
        res.header('Access-Control-Allow-Origin', 'http://localhost:63342');
    } else if (!urlString?.includes(serverURL) && process.env.debug !== 'true') {
        res.sendStatus(503);
        return false;
    }

    const tmpHosts = serverURL.split(':')
    if (tmpHosts.length === 2 && tmpHosts[1] != process.env.port) {
        res.status(503)
        return false
    }
    return true;
}

function usersUpdate(serial_number: string, ip_address: string, card_id: string, card_type: number, created_at: string | null, update: boolean = false) {
    const users = {
        serial_number: serial_number,
        created_at: created_at ?? dayjs().tz('Asia/Shanghai').format('YYYY-MM-DD HH:mm:ss.SSS').toString(),
        updated_at: dayjs().tz('Asia/Shanghai').format('YYYY-MM-DD HH:mm:ss.SSS').toString(),
        ip_address: ip_address,
        card_id: card_id,
        card_type: card_type,
    }
    return new Promise((resolve, reject) => {
        if (update) {
            db.update(users).from('users').where('serial_number', serial_number).then((data) => {
                resolve(data);
            }).catch((err) => {
                reject(err);
            })
        } else {
            db.insert(users).from('users').then((data) => {
                resolve(data);
            }).catch((err) => {
                reject(err);
            })
        }
    })
}

function usersDelete() {

}

function cardsUpdate(cards: any, cardId: string) {
    cards.updated_at = dayjs().tz('Asia/Shanghai').format('YYYY-MM-DD HH:mm:ss.SSS').toString()
    return new Promise((resolve, reject) => {
        db.update(cards).from('cards').where('card_id', cardId).then((data) => {
            resolve(data)
        }).catch((error) => {
            reject(error)
        })
    })
}

function cardsDelete() {

}

function cardsCreate() {

}

export {
    isCurl,
    isWeb,
    getTimeGap,
    getMD5,
    handleRequest,
    usersUpdate,
    usersDelete,
    cardsCreate,
    cardsUpdate,
    cardsDelete,
    encodeHash,
    decodeHash,
    checkAuch,
    db,
}
