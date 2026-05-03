'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

async function loadUtils() {
  return import('../../webui/src/features/chatHistory/chatHistoryUtils.js');
}

test('chat history strict parser merges current input file placeholder', async () => {
  const {
    buildListModeMessages,
  } = await loadUtils();
  const t = (key) => key;
  const item = {
    messages: [{
      role: 'user',
      content: 'Continue from the latest state in the attached DS2API_HISTORY.txt context. Treat it as the current working state and answer the latest user request directly.',
    }],
    history_text: [
      '<｜begin▁of▁sentence｜>',
      '<｜User｜>hello',
      '<｜Assistant｜>hi<｜end▁of▁sentence｜>',
    ].join(''),
  };

  const result = buildListModeMessages(item, t);
  assert.equal(result.historyMerged, true);
  assert.deepEqual(result.messages, [
    { role: 'user', content: 'hello' },
    { role: 'assistant', content: 'hi' },
  ]);
});

test('chat history strict parser inserts history after system messages', async () => {
  const {
    buildListModeMessages,
  } = await loadUtils();
  const t = (key) => key;
  const item = {
    messages: [
      { role: 'system', content: 'policy' },
      { role: 'user', content: 'latest' },
    ],
    history_text: [
      '<｜begin▁of▁sentence｜>',
      '<｜User｜>old',
      '<｜Assistant｜>done<｜end▁of▁sentence｜>',
    ].join(''),
  };

  const result = buildListModeMessages(item, t);
  assert.equal(result.historyMerged, true);
  assert.deepEqual(result.messages, [
    { role: 'system', content: 'policy' },
    { role: 'user', content: 'old' },
    { role: 'assistant', content: 'done' },
    { role: 'user', content: 'latest' },
  ]);
});
