export const QUICK_TEST_PROMPTS = [
  '今天几号',
  '几点了',
  '天气呢',
  '说个颜色',
  '讲个笑话',
  '推荐电影',
  '推荐歌单',
  '推荐早餐',
  '推荐晚餐',
  '推荐饮品',
  '写句诗',
  '写标题',
  '写口号',
  '写祝福',
  '写摘要',
  '翻译早安',
  '翻译谢谢',
  '解释云朵',
  '解释海浪',
  '解释星空',
  '列三点',
  '列五项',
  '问个问题',
  '给个建议',
  '给个昵称',
  '给个名字',
  '给个比喻',
  '给个灵感',
  '随便聊聊',
  '夸我一句',
  '安慰一句',
  '祝我顺利',
  '讲个成语',
  '讲个典故',
  '讲个冷知',
  '讲个趣事',
  '讲个故事',
  '聊聊跑步',
  '聊聊睡眠',
  '聊聊咖啡',
  '聊聊旅行',
  '聊聊编程',
  '聊聊效率',
  '聊聊阅读',
  '聊聊音乐',
  '聊聊摄影',
  '聊聊美食',
  '聊聊宠物',
  '聊聊春天',
  '聊聊周末',
];

export function pickRandomQuickTestPrompt() {
  if (!Array.isArray(QUICK_TEST_PROMPTS) || QUICK_TEST_PROMPTS.length === 0) {
    return '简单介绍';
  }
  const index = Math.floor(Math.random() * QUICK_TEST_PROMPTS.length);
  return QUICK_TEST_PROMPTS[index] || QUICK_TEST_PROMPTS[0];
}

export function buildQuickTestMessages() {
  return [{ role: 'user', content: pickRandomQuickTestPrompt() }];
}
