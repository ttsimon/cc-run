import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/cc-run/',
  lang: 'zh-CN',
  title: 'ccr',
  description: '用选定 provider 的环境变量启动 claude——多开各用不同后端、互不干扰、不改全局配置。',
  lastUpdated: true,
  cleanUrls: true,

  // i18n：root = 简体中文（挂 /，无前缀）。将来加英文：在此加 en locale + website/en/。
  locales: {
    root: { label: '简体中文', lang: 'zh-CN' }
    // en: { label: 'English', lang: 'en', link: '/en/' }  // 预留，本期不写
  },

  themeConfig: {
    nav: [
      { text: '指南', link: '/guide/what-is-ccr' },
      { text: 'chain', link: '/chain/concept' },
      { text: '参考', link: '/reference/commands' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: '指南',
          items: [
            { text: '这是什么', link: '/guide/what-is-ccr' },
            { text: '安装', link: '/guide/installation' },
            { text: '快速上手', link: '/guide/getting-started' },
            { text: '配置与按名解析', link: '/guide/profiles' },
            { text: 'doctor 体检', link: '/guide/doctor' }
          ]
        }
      ],
      '/chain/': [
        {
          text: 'chain 多后端流水线',
          items: [
            { text: '概念', link: '/chain/concept' },
            { text: '写一条链', link: '/chain/writing-a-chain' },
            { text: '放行与审查', link: '/chain/pausing' },
            { text: '隔离与成果交回', link: '/chain/isolation' },
            { text: '安全', link: '/chain/security' }
          ]
        }
      ],
      '/reference/': [
        {
          text: '参考',
          items: [
            { text: '命令速查', link: '/reference/commands' },
            { text: '配置文件与路径', link: '/reference/files' },
            { text: 'FAQ / 故障排查', link: '/reference/faq' }
          ]
        }
      ]
    },
    search: {
      provider: 'local',
      options: {
        // 中文可搜：放宽分词，按单字切分兜底
        miniSearch: {
          options: {
            tokenize: (text: string) => text.split(/[\s\-/]+|(?<=[一-龥])(?=[一-龥])/)
          }
        }
      }
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/ttsimon/cc-run' }
    ],
    editLink: {
      pattern: 'https://github.com/ttsimon/cc-run/edit/master/website/:path',
      text: '在 GitHub 上编辑此页'
    },
    docFooter: { prev: '上一页', next: '下一页' },
    outline: { label: '本页目录' },
    lastUpdatedText: '最后更新'
  }
})
