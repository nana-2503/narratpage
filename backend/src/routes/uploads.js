import { Router } from 'express';
import multer from 'multer';
import { randomBytes } from 'node:crypto';
import { mkdirSync } from 'node:fs';
import { readFile, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { requireAuth } from '../auth.js';
import { DATA_DIR } from '../db.js';

export const uploadsRouter = Router();

export const UPLOAD_DIR = join(DATA_DIR, 'uploads');
mkdirSync(UPLOAD_DIR, { recursive: true });
const MAX_SIZE = 5 * 1024 * 1024; // 5MB

/** 扩展名映射（按声明 MIME） */
const EXT_BY_MIME = new Map([
  ['image/png', '.png'],
  ['image/jpeg', '.jpg'],
  ['image/webp', '.webp'],
  ['image/gif', '.gif'],
]);

/** 文件头 magic bytes 校验：MIME 由客户端声明、不可信，必须以内容为准 */
async function looksLikeImage(filePath) {
  const buf = await readFile(filePath).then((b) => b.subarray(0, 12));
  const hex = (offset, length) => buf.subarray(offset, offset + length).toString('hex');
  return (
    hex(0, 8) === '89504e470d0a1a0a' || // PNG
    hex(0, 3) === 'ffd8ff' || // JPEG
    buf.subarray(0, 4).toString() === 'GIF8' || // GIF87a/GIF89a
    (hex(0, 4) === '52494646' && buf.subarray(8, 12).toString() === 'WEBP') // WebP
  );
}

const upload = multer({
  storage: multer.diskStorage({
    destination: UPLOAD_DIR,
    filename: (_req, file, cb) => {
      // 随机文件名：不信任用户文件名（防路径穿越与覆盖）
      const ext = EXT_BY_MIME.get(file.mimetype) ?? '';
      cb(null, `${randomBytes(8).toString('hex')}${ext}`);
    },
  }),
  limits: { fileSize: MAX_SIZE },
  fileFilter: (_req, file, cb) => {
    if (!EXT_BY_MIME.has(file.mimetype)) {
      const err = new Error('仅支持 PNG / JPEG / WebP / GIF 图片');
      err.status = 400;
      return cb(err);
    }
    cb(null, true);
  },
});

// POST /api/uploads — 上传图片（仅管理员），返回可直接用于 Markdown 的相对地址
uploadsRouter.post('/', requireAuth, upload.single('file'), async (req, res, next) => {
  try {
    if (!req.file) {
      return res.status(400).json({ error: '未收到文件' });
    }
    if (!(await looksLikeImage(req.file.path))) {
      await unlink(req.file.path);
      return res.status(400).json({ error: '文件内容不是有效图片' });
    }
    res.status(201).json({ url: `/api/uploads/${req.file.filename}` });
  } catch (err) {
    next(err);
  }
});
