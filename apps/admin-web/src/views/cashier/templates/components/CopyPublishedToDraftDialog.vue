<template>
  <el-dialog
    :model-value="open"
    title="复制 published 为 draft"
    width="520px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="published 版本只读，复制后会生成新的 draft，且与原版本不再联动。"
    />
    <p class="desc">来源版本：v{{ sourceVersion.version }}（{{ sourceVersion.status }}）</p>
    <template #footer>
      <el-button @click="emit('close')">取消</el-button>
      <el-button v-perm="'cashier.write'" type="primary" :loading="submitting" @click="submit">确认复制</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { copyTemplateVersionToDraft, type CashierTemplateVersion } from "@/api/modules/cashier";

const props = defineProps<{
  open: boolean;
  templateId: string;
  sourceVersion: CashierTemplateVersion;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "created"): void;
}>();

const submitting = ref(false);

async function submit() {
  submitting.value = true;
  try {
    const created = await copyTemplateVersionToDraft(props.templateId, props.sourceVersion.version);
    ElMessage.success(`已复制为 draft 版本 v${created.version}`);
    emit("created");
    emit("close");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "复制版本失败");
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.desc {
  margin: 12px 0 0;
  color: var(--text-subtle);
}
</style>
