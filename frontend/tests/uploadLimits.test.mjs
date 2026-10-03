import assert from 'node:assert/strict'
import test from 'node:test'
import { MAX_UPLOAD_SIZE_BYTES, isUploadFileTooLarge, uploadSizeLimitMessage } from '../src/shared/files/uploadLimits.ts'

test('upload boundary matches the backend 20 MiB processing limit', () => {
  assert.equal(MAX_UPLOAD_SIZE_BYTES, 20 * 1024 * 1024)
  assert.equal(isUploadFileTooLarge({ size: MAX_UPLOAD_SIZE_BYTES }), false)
  assert.equal(isUploadFileTooLarge({ size: MAX_UPLOAD_SIZE_BYTES + 1 }), true)
  assert.equal(uploadSizeLimitMessage(), '单个文件不能超过 20.0 MB')
})
