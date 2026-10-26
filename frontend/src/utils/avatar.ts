/**
 * 头像相关的预设清单与纯函数。
 *
 * 预设图放在 `public/avatars/users/` 而不是 `src/assets/`：这里的每一项都会被
 * 当成字符串写进 avatarUrl 落库。assets 目录下的文件经过构建会带内容哈希，
 * 重新部署后哈希变化，库里存的旧地址就全部 404 了。public 目录原样复制，
 * 路径与构建无关，这是选它的唯一理由。
 */

/**
 * 可直接点选的预设头像，值就是原样存进 avatarUrl 的路径。
 *
 * p1–p3 是手选的插画；p4–p10 取自 loliapi 的随机头像接口，抓下来后**离线固化**。
 * 不把接口地址 `https://www.loliapi.com/acg/pp/` 直接存进 avatarUrl，有两个硬理由：
 *   1. 它是随机的 —— 每次渲染都换一张脸，头像会不停变，用户没法「选定」；
 *   2. 它 302 到 eo-img.* 的 CDN，而落地域名会轮换（实测见过 loliapi.com /
 *      loliapi.cn / iloli.love / 544521.xyz / 20240828.xyz），存下来的地址
 *      过一阵可能指向已经不存在的域名。
 * 抓取、裁切、压缩都在离线做，仓库里只留成品，运行时不依赖这个接口。
 */
export const USER_AVATAR_PRESETS: readonly string[] = [
  '/avatars/users/p1.jpg',
  '/avatars/users/p2.jpg',
  '/avatars/users/p3.jpg',
  '/avatars/users/p4.jpg',
  '/avatars/users/p5.jpg',
  '/avatars/users/p6.jpg',
  '/avatars/users/p7.jpg',
  '/avatars/users/p8.jpg',
  '/avatars/users/p9.jpg',
  '/avatars/users/p10.jpg',
]

/**
 * 名字为空时也得有个字符，否则圆形区域里是空的。
 * 用 charAt 而不是 [0]：空串下标取值得到 undefined，会渲染成字面量 "undefined"。
 */
export function avatarInitialOf(name: string | null | undefined): string {
  const trimmed = (name ?? '').trim()

  return trimmed === '' ? '?' : trimmed.charAt(0).toUpperCase()
}

const FALLBACK_COLORS = ['#5b8def', '#e08a5f', '#4fa87a', '#9a6fd0', '#3f9fb8', '#cf6c8b'] as const

/**
 * 按名字取底色：同一个名字永远同一色，刷新页面不会换。
 * 取模前先取绝对值 —— 手写 hash 可能算出负数，负数下标取不到颜色。
 */
export function avatarColorOf(name: string | null | undefined): string {
  const text = (name ?? '').trim()
  let hash = 0

  for (let i = 0; i < text.length; i += 1) {
    hash = (hash * 31 + text.charCodeAt(i)) % 100000
  }

  return FALLBACK_COLORS[Math.abs(hash) % FALLBACK_COLORS.length]
}
