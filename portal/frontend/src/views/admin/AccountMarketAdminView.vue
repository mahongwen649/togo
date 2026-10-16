<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header flex items-center justify-between gap-3">
        <h1 class="page-title">账号业务管理</h1>
        <div class="flex gap-2">
          <button
            class="btn btn-secondary"
            title="刷新"
            aria-label="刷新"
            @click="load"
          >
            <Icon name="refresh" size="sm" /></button
          ><button class="btn btn-primary" @click="newProduct">
            <Icon name="plus" size="sm" />新增商品
          </button>
        </div>
      </header>
      <p
        v-if="notice"
        class="text-sm"
        :class="error ? 'text-red-600' : 'text-emerald-600'"
      >
        {{ notice }}
      </p>
      <section class="space-y-3">
        <h2 class="text-base font-semibold">商品</h2>
        <div
          class="overflow-x-auto border-y border-gray-200 dark:border-dark-700"
        >
          <table class="min-w-full text-left text-sm">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3">商品</th>
                <th class="px-4 py-3">业务</th>
                <th class="px-4 py-3">单价</th>
                <th class="px-4 py-3">可售 / 锁定</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-for="p in products" :key="p.id">
                <td class="px-4 py-3 font-medium">{{ p.name }}</td>
                <td class="px-4 py-3">
                  {{ productBusinessLabel(p) }}
                </td>
                <td class="px-4 py-3">¥{{ money(p.unit_price) }}</td>
                <td class="px-4 py-3">
                  {{ p.stock_quantity - p.locked_quantity }} /
                  {{ p.locked_quantity }}
                </td>
                <td class="px-4 py-3">{{ p.enabled ? "上架" : "下架" }}</td>
                <td class="px-4 py-3 text-right">
                  <button
                    class="btn btn-secondary btn-sm"
                    title="编辑商品"
                    aria-label="编辑商品"
                    @click="editProduct(p)"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <p
            v-if="!products.length"
            class="p-8 text-center text-sm text-gray-500"
          >
            暂无商品
          </p>
        </div>
      </section>
      <section class="space-y-3">
        <div class="flex items-center justify-between">
          <h2 class="text-base font-semibold">订单处理</h2>
          <select
            v-model="statusFilter"
            class="input w-32"
            aria-label="订单状态"
          >
            <option value="">全部订单</option>
            <option value="pending">待处理</option>
            <option value="completed">已完成</option>
            <option value="cancelled">已取消</option>
            <option value="expired">已过期</option>
          </select>
        </div>
        <div
          class="overflow-x-auto border-y border-gray-200 dark:border-dark-700"
        >
          <table class="min-w-full text-left text-sm">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3">订单 / 用户</th>
                <th class="px-4 py-3">商品</th>
                <th class="px-4 py-3">数量 / 金额</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">分组</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-for="o in filteredOrders" :key="o.id">
                <td class="px-4 py-3">
                  #{{ o.id }}
                  <div class="text-xs text-gray-500">用户 #{{ o.user_id }}</div>
                </td>
                <td class="px-4 py-3">{{ o.product_name }}</td>
                <td class="px-4 py-3">
                  {{ o.quantity }} 个 / ¥{{ money(o.total_price) }}
                </td>
                <td
                  class="px-4 py-3"
                  :class="o.fulfillment_error ? 'text-red-600' : ''"
                >
                  {{ statusText(o) }}
                </td>
                <td class="px-4 py-3">
                  {{ o.core_group_id ? `#${o.core_group_id}` : "待分配" }}
                </td>
                <td class="px-4 py-3 text-right">
                  <template v-if="o.status === 'pending'"
                    ><button
                      v-if="o.mode === 'rental'"
                      class="btn btn-primary btn-sm"
                      :disabled="busy !== null"
                      @click="openFulfillment(o)"
                    >
                      {{
                        o.fulfillment_started_at ? "重试同步" : "分配账号"
                      }}</button
                    ><button
                      class="btn btn-secondary btn-sm ml-2"
                      :disabled="busy !== null || !!o.fulfillment_started_at"
                      @click="cancelTarget = o"
                    >
                      取消退款
                    </button></template
                  ><span v-else class="text-xs text-gray-500">{{
                    o.core_account_ids?.map((id) => `#${id}`).join(", ") || "-"
                  }}</span>
                </td>
              </tr>
            </tbody>
          </table>
          <p
            v-if="!filteredOrders.length"
            class="p-8 text-center text-sm text-gray-500"
          >
            暂无订单
          </p>
        </div>
      </section>
      <BaseDialog
        :show="editing"
        :title="form.id ? '编辑商品' : '新增商品'"
        @close="!saving && (editing = false)"
        ><form
          id="product-form"
          class="space-y-4"
          @submit.prevent="saveProduct"
        >
          <p v-if="formError" class="text-sm text-red-600">{{ formError }}</p>
          <label class="block space-y-1 text-sm"><span>商品名称</span><input
            v-model="form.name"
            class="input w-full"
            placeholder="商品名称"
            required
            maxlength="100"
          /></label>
          <fieldset class="space-y-2 text-sm">
            <legend>业务类型</legend>
            <div class="grid grid-cols-2 gap-2">
              <label
                class="flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-3 transition-colors"
                :class="form.mode === 'rental' ? 'border-primary-500 bg-primary-50 text-primary-800 dark:bg-primary-950/30 dark:text-primary-200' : 'border-gray-200 dark:border-dark-600'"
              >
                <input v-model="form.mode" class="sr-only" type="radio" value="rental" @change="handleModeChange" />
                <span class="flex-1"><span class="block font-semibold">租号</span><span class="mt-0.5 block text-xs opacity-70">按有效期交付账号使用权</span></span>
              </label>
              <label
                class="flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-3 transition-colors"
                :class="form.mode === 'purchase' ? 'border-primary-500 bg-primary-50 text-primary-800 dark:bg-primary-950/30 dark:text-primary-200' : 'border-gray-200 dark:border-dark-600'"
              >
                <input v-model="form.mode" class="sr-only" type="radio" value="purchase" @change="handleModeChange" />
                <span class="flex-1"><span class="block font-semibold">账号</span><span class="mt-0.5 block text-xs opacity-70">按账号数量一次性购买</span></span>
              </label>
            </div>
          </fieldset>
          <label v-if="form.mode === 'rental'" class="block space-y-1 text-sm">
            <span>租号有效期（天）</span>
            <input
              v-model.number="form.validity_days"
              type="number"
              min="1"
              max="3650"
              step="1"
              list="account-product-validity-presets"
              class="input w-full"
              placeholder="例如 30"
              required
            />
            <datalist id="account-product-validity-presets">
              <option value="7" />
              <option value="15" />
              <option value="30" />
              <option value="90" />
              <option value="180" />
              <option value="365" />
            </datalist>
            <span class="block text-xs text-gray-500 dark:text-dark-400">仅 30 天租号套餐支持用户开启自动续费。</span>
          </label>
          <div class="grid grid-cols-2 gap-3">
            <label class="block min-w-0 space-y-1 text-sm"><span>单价（元）</span><input
              v-model.number="form.unit_price"
              type="number"
              min="0"
              step="0.01"
              class="input w-full"
              placeholder="单价"
              required
            /></label><label class="block min-w-0 space-y-1 text-sm"><span>库存（含锁定）</span><input
              v-model.number="form.stock_quantity"
              type="number"
              :min="editingLocked"
              class="input w-full"
              placeholder="库存"
              required
            /></label>
          </div>
          <div class="space-y-2 text-sm">
            <span class="block">广告图</span>
            <div class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800">
              <div class="aspect-square w-full">
                <img v-if="form.image_url" :src="form.image_url" alt="广告图预览" class="h-full w-full object-contain" />
                <div v-else class="flex h-full items-center justify-center text-sm text-gray-400">尚未上传广告图</div>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 px-3 py-2 dark:border-dark-600">
                <span class="text-xs text-gray-500 dark:text-dark-400">JPG、PNG、WebP 或 GIF，保存前自动压缩至 600KB 内</span>
                <div class="flex gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer" :class="imageProcessing ? 'pointer-events-none opacity-60' : ''">
                    <Icon name="upload" size="sm" />{{ imageProcessing ? "处理中..." : form.image_url ? "更换图片" : "上传图片" }}
                    <input class="sr-only" type="file" accept="image/jpeg,image/png,image/webp,image/gif" :disabled="imageProcessing" @change="handleProductImageChange" />
                  </label>
                  <button v-if="form.image_url" type="button" class="btn btn-secondary btn-sm" :disabled="imageProcessing" title="移除广告图" aria-label="移除广告图" @click="form.image_url = ''">
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
            </div>
          </div><label class="block space-y-1 text-sm"><span>商品说明</span><textarea
            v-model="form.description"
            class="input min-h-20 w-full"
            placeholder="商品说明"
          /></label><label class="block space-y-1 text-sm"><span>购买须知</span><textarea
            v-model="form.purchase_notice"
            class="input min-h-20 w-full"
            placeholder="购买须知"
          /></label><label class="flex items-center gap-2 text-sm"
            ><input v-model="form.enabled" type="checkbox" />上架</label
          >
        </form>
        <template #footer
          ><button class="btn btn-secondary" :disabled="saving" @click="editing = false">
            取消</button
          ><button
            form="product-form"
            class="btn btn-primary"
            :disabled="saving || imageProcessing"
          >
            {{ saving ? "保存中..." : "保存" }}
          </button></template
        ></BaseDialog
      >
      <BaseDialog
        :show="!!fulfillment"
        title="分配租赁账号"
        width="wide"
        @close="busy === null && (fulfillment = null)"
        ><div v-if="fulfillment" class="space-y-4">
          <div class="flex justify-between text-sm">
            <span
              >订单 #{{ fulfillment.id }} · {{ fulfillment.product_name }}</span
            ><span
              >已选 {{ selected.length }} / {{ fulfillment.quantity }}</span
            >
          </div>
          <p v-if="fulfillmentError" class="text-sm text-red-600">
            {{ fulfillmentError }}
          </p>
          <form
            class="flex gap-2"
            @submit.prevent="
              accountPage = 1;
              loadAccounts();
            "
          >
            <input
              v-model="accountSearch"
              class="input flex-1"
              placeholder="模糊搜索账号名称或 ID"
              aria-label="模糊搜索账号名称或 ID"
            /><button class="btn btn-secondary" title="搜索账号" aria-label="搜索账号">
              <Icon name="search" size="sm" />
            </button>
          </form>
          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex flex-col gap-2 sm:flex-row">
              <label class="sr-only" for="account-picker">选择 Core 账号</label>
              <select
                id="account-picker"
                v-model="accountCandidate"
                class="input min-w-0 flex-1"
                :disabled="busy !== null || !!fulfillment.fulfillment_started_at || selected.length >= fulfillment.quantity"
              >
                <option :value="null">
                  {{ accountOptions.length ? "选择可分配的 Core 账号" : "暂无可分配账号" }}
                </option>
                <option v-for="a in accountOptions" :key="a.id" :value="a.id">
                  {{ accountDisplayName(a) }} · #{{ a.id }} · {{ a.platform }}
                </option>
              </select>
              <button
                type="button"
                class="btn btn-primary sm:w-24"
                :disabled="busy !== null || !!fulfillment.fulfillment_started_at || accountCandidate === null || !candidateAccount?.available || selected.length >= fulfillment.quantity"
                @click="addAccount"
              >
                添加账号
              </button>
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              仅显示可分配账号。可输入账号名称或 ID 后点击搜索，已选 {{ selected.length }} 个，还需选择 {{ Math.max(0, fulfillment.quantity - selected.length) }} 个。
            </p>
          </div>
          <div v-if="selectedAccounts.length" class="space-y-2">
            <div class="text-sm font-medium text-gray-900 dark:text-white">已选择账号</div>
            <div class="divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-800 dark:border-dark-700">
              <div v-for="a in selectedAccounts" :key="a.id" class="flex items-center gap-3 px-3 py-2.5 text-sm">
                <span class="min-w-0 flex-1 truncate">
                  <span class="font-medium text-gray-900 dark:text-white">{{ accountDisplayName(a) }}</span>
                  <span class="ml-2 font-mono text-xs text-gray-400">#{{ a.id }}</span>
                  <span class="ml-2 text-xs text-gray-500">{{ a.platform }}</span>
                </span>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="busy !== null || !!fulfillment.fulfillment_started_at"
                  title="移除账号"
                  :aria-label="`移除 ${accountDisplayName(a)}`"
                  @click="removeAccount(a.id)"
                >
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>
          </div>
          <p v-else class="rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-600">
            尚未选择账号
          </p>
          <div v-if="!accounts.length" class="text-xs text-gray-500 dark:text-dark-400">
            当前搜索条件下没有可分配账号，请修改名称或 ID 后重新搜索。
          </div>
          <div class="flex justify-between text-xs text-gray-500">
            <span>共 {{ accountTotal }} 个账号</span
            ><span
              ><button
                class="btn btn-secondary btn-sm mr-2"
                :disabled="accountPage <= 1"
                @click="
                  accountPage--;
                  loadAccounts();
                "
              >
                上一页</button
              ><button
                class="btn btn-secondary btn-sm"
                :disabled="accountPage * 50 >= accountTotal"
                @click="
                  accountPage++;
                  loadAccounts();
                "
              >
                下一页
              </button></span
            >
          </div>
        </div>
        <template #footer
          ><button class="btn btn-secondary" :disabled="busy!==null" @click="fulfillment = null">
            关闭</button
          ><button
            class="btn btn-primary"
            :disabled="
              busy !== null || selected.length !== fulfillment?.quantity
            "
            @click="completeOrder"
          >
            {{ busy !== null ? "同步中..." : "同步并完成" }}
          </button></template
        ></BaseDialog
      >
      <BaseDialog
        :show="!!cancelTarget"
        title="取消订单"
        @close="busy === null && (cancelTarget = null)"
        ><p class="text-sm">
          订单 #{{ cancelTarget?.id }} 将退款 ¥{{
            money(cancelTarget?.total_price || 0)
          }}。
        </p>
        <template #footer
          ><button class="btn btn-secondary" @click="cancelTarget = null">
            返回</button
          ><button
            class="btn btn-primary"
            :disabled="busy !== null"
            @click="cancelOrder"
          >
            确认退款
          </button></template
        ></BaseDialog
      >
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import AppLayout from "@/components/layout/AppLayout.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import Icon from "@/components/icons/Icon.vue";
import { apiClient } from "@/api/client";
type Product = {
  id: number;
  name: string;
  mode: "rental" | "purchase";
  validity_days: number;
  unit_price: number;
  stock_quantity: number;
  locked_quantity: number;
  image_url: string;
  description: string;
  purchase_notice: string;
  enabled: boolean;
  sort_order: number;
};
type Order = {
  id: number;
  user_id: number;
  product_name: string;
  mode: "rental" | "purchase";
  validity_days: number;
  quantity: number;
  total_price: number;
  status: string;
  core_account_ids: number[];
  core_group_id: number | null;
  expires_at: string | null;
  fulfillment_started_at: string | null;
  fulfillment_error: string;
};
type Account = {
  id: number;
  name: string;
  platform: string;
  available: boolean;
};
const products = ref<Product[]>([]),
  orders = ref<Order[]>([]),
  accounts = ref<Account[]>([]),
  loading = ref(false),
  notice = ref(""),
  error = ref(false),
  statusFilter = ref(""),
  busy = ref<number | null>(null),
  editing = ref(false),
  saving = ref(false),
  imageProcessing = ref(false),
  formError = ref(""),
  editingLocked = ref(0),
  fulfillment = ref<Order | null>(null),
  selected = ref<number[]>([]),
  fulfillmentError = ref(""),
  cancelTarget = ref<Order | null>(null),
  accountSearch = ref(""),
  accountCandidate = ref<number | null>(null),
  accountPage = ref(1),
  accountTotal = ref(0);
const blank = () => ({
    id: 0,
    name: "",
    mode: "rental" as "rental" | "purchase",
    validity_days: 30,
    unit_price: 0,
    stock_quantity: 0,
    image_url: "",
    description: "",
    purchase_notice: "",
    enabled: true,
    sort_order: 0,
  }),
  form = reactive(blank()),
  filteredOrders = computed(() =>
    orders.value.filter(
      (o) => !statusFilter.value || o.status === statusFilter.value,
    ),
  ),
  accountOptions = computed(() =>
    accounts.value.filter((account) => account.available && !selected.value.includes(account.id)),
  ),
  candidateAccount = computed(() => accounts.value.find((account) => account.id === accountCandidate.value)),
  selectedAccounts = computed(() =>
    selected.value.map((id) => accounts.value.find((account) => account.id === id) || {
      id,
      name: "账号",
      platform: "Core",
      available: true,
    }),
  ),
  money = (n: number) => Number(n).toFixed(2),
  message = (e: unknown) =>
    (e as { message?: string })?.message || "请求失败，请稍后重试";
function statusText(o: Order) {
  return o.status === "pending" && o.fulfillment_error
    ? "同步待重试"
    : o.status === "pending" && o.fulfillment_started_at
      ? "处理中"
      : (
          {
            pending: "待处理",
            completed: "已完成",
            cancelled: "已取消",
            expired: "已过期",
          } as Record<string, string>
        )[o.status] || o.status;
}
function accountDisplayName(account: Account) {
  return account.name?.trim() || "未命名账号";
}
function productBusinessLabel(product: Pick<Product, "mode" | "validity_days">) {
  return product.mode === "rental" ? `租号 · ${product.validity_days} 天` : "账号";
}
function handleModeChange() {
  form.validity_days = form.mode === "rental" ? Math.max(1, form.validity_days || 30) : 0;
}
function readBlobAsDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === "string" ? reader.result : "");
    reader.onerror = () => reject(reader.error || new Error("广告图读取失败"));
    reader.readAsDataURL(blob);
  });
}
function loadProductImage(source: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("广告图读取失败"));
    image.src = source;
  });
}
function canvasToProductBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => blob ? resolve(blob) : reject(new Error("广告图压缩失败")), "image/webp", quality);
  });
}
async function prepareProductImage(file: File): Promise<string> {
  const maxInputBytes = 10 * 1024 * 1024;
  const maxSavedBytes = 600 * 1024;
  if (!file.type.startsWith("image/") || !["image/jpeg", "image/png", "image/webp", "image/gif"].includes(file.type)) {
    throw new Error("广告图仅支持 JPG、PNG、WebP 或 GIF");
  }
  if (file.size > maxInputBytes) {
    throw new Error("广告图原文件不能超过 10MB");
  }
  if (file.type === "image/gif") {
    if (file.size > maxSavedBytes) throw new Error("GIF 广告图不能超过 600KB");
    return readBlobAsDataURL(file);
  }
  const source = await readBlobAsDataURL(file);
  const image = await loadProductImage(source);
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) throw new Error("当前浏览器无法处理广告图");
  const baseScale = Math.min(1, 1600 / image.naturalWidth, 900 / image.naturalHeight);
  const scaleSteps = [1, 0.85, 0.7, 0.55, 0.4, 0.3];
  const qualitySteps = [0.9, 0.82, 0.74, 0.66, 0.58, 0.5];
  for (const scaleStep of scaleSteps) {
    const scale = baseScale * scaleStep;
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
    context.clearRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 0, 0, canvas.width, canvas.height);
    for (const quality of qualitySteps) {
      const blob = await canvasToProductBlob(canvas, quality);
      if (blob.size <= maxSavedBytes) return readBlobAsDataURL(blob);
    }
  }
  throw new Error("广告图压缩后仍超过 600KB，请选择更小的图片");
}
async function handleProductImageChange(event: Event) {
  const input = event.target as HTMLInputElement | null;
  const file = input?.files?.[0];
  if (input) input.value = "";
  if (!file || imageProcessing.value) return;
  imageProcessing.value = true;
  formError.value = "";
  try {
    form.image_url = await prepareProductImage(file);
  } catch (e) {
    formError.value = message(e);
  } finally {
    imageProcessing.value = false;
  }
}
async function load() {
  loading.value = true;
  try {
    const [p, o] = await Promise.all([
      apiClient.get<{ items: Product[] }>("/admin/account-market/products"),
      apiClient.get<{ items: Order[] }>("/admin/account-market/orders"),
    ]);
    products.value = p.data.items || [];
    orders.value = o.data.items || [];
  } catch (e) {
    notice.value = message(e);
    error.value = true;
  } finally {
    loading.value = false;
  }
}
function newProduct() {
  Object.assign(form, blank());
  editingLocked.value = 0;
  formError.value = "";
  editing.value = true;
}
function editProduct(p: Product) {
  const { locked_quantity, ...fields } = p;
  Object.assign(form, fields, { validity_days: p.mode === "rental" ? p.validity_days || 30 : 0 });
  editingLocked.value = locked_quantity;
  formError.value = "";
  editing.value = true;
}
async function saveProduct() {
  formError.value = "";
  if (form.mode === "rental" && (!Number.isInteger(form.validity_days) || form.validity_days < 1 || form.validity_days > 3650)) {
    formError.value = "租号有效期必须是 1 到 3650 天之间的整数";
    return;
  }
  if (imageProcessing.value) return;
  saving.value = true;
  try {
    await apiClient.post("/admin/account-market/products", form);
    editing.value = false;
    notice.value = "商品已保存";
    error.value = false;
    await load();
  } catch (e) {
    formError.value = message(e);
  } finally {
    saving.value = false;
  }
}
async function openFulfillment(o: Order) {
  fulfillment.value = o;
  selected.value = [...(o.core_account_ids || [])];
  accountPage.value = 1;
  accountSearch.value = "";
  accountCandidate.value = null;
  fulfillmentError.value = "";
  await loadAccounts();
}
async function loadAccounts() {
  try {
    const { data } = await apiClient.get<{ items: Account[]; total: number }>(
      "/admin/account-market/core-accounts",
      {
        params: {
          page: accountPage.value,
          search: accountSearch.value,
          order_id: fulfillment.value?.id,
        },
      },
    );
    accounts.value = data.items || [];
    accountTotal.value = data.total || 0;
    if (accountCandidate.value !== null && !accountOptions.value.some((account) => account.id === accountCandidate.value)) {
      accountCandidate.value = null;
    }
  } catch (e) {
    accounts.value = [];
    fulfillmentError.value = message(e);
  }
}
function addAccount() {
  const o = fulfillment.value;
  const id = accountCandidate.value;
  if (!o || id === null || selected.value.length >= o.quantity || selected.value.includes(id)) return;
  const account = accounts.value.find((item) => item.id === id);
  if (!account?.available) return;
  selected.value = [...selected.value, id];
  accountCandidate.value = null;
}
function removeAccount(id: number) {
  if (fulfillment.value?.fulfillment_started_at) return;
  selected.value = selected.value.filter((item) => item !== id);
}
async function completeOrder() {
  const o = fulfillment.value;
  if (!o || busy.value !== null || selected.value.length !== o.quantity) return;
  busy.value = o.id;
  try {
    await apiClient.put(
      `/admin/account-market/orders/${o.id}`,
      { status: "completed", core_account_ids: selected.value },
      { timeout: 115000 },
    );
    fulfillment.value = null;
    notice.value = "账号已同步，订单已完成";
    error.value = false;
    await load();
  } catch (e) {
    fulfillmentError.value = message(e);
    await load();
    fulfillment.value = orders.value.find((v) => v.id === o.id) || o;
    if (fulfillment.value.fulfillment_started_at)
      selected.value = [...fulfillment.value.core_account_ids];
  } finally {
    busy.value = null;
  }
}
async function cancelOrder() {
  const o = cancelTarget.value;
  if (!o || busy.value !== null) return;
  busy.value = o.id;
  try {
    await apiClient.put(`/admin/account-market/orders/${o.id}`, {
      status: "cancelled",
    });
    cancelTarget.value = null;
    notice.value = "订单已取消，余额已退回";
    error.value = false;
    await load();
  } catch (e) {
    notice.value = message(e);
    error.value = true;
  } finally {
    busy.value = null;
  }
}
onMounted(load);
</script>
