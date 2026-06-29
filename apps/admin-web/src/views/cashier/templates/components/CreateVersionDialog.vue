<template>
  <el-dialog
    :model-value="open"
    title="新建 draft 版本"
    width="520px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <el-form :model="form" label-position="top">
      <el-form-item label="来源类型">
        <el-radio-group v-model="form.sourceType">
          <el-radio value="manual">manual（空白版本）</el-radio>
          <el-radio value="copy_published">copy_published</el-radio>
          <el-radio value="copy_archived">copy_archived</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="来源版本" :required="form.sourceType !== 'manual'">
        <el-input
          v-model.trim="form.sourceVersion"
          :placeholder="form.sourceType === 'manual' ? 'manual 可留空' : '如：8'"
          :disabled="form.sourceType === 'manual'"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('close')">取消</el-button>
      <el-button v-perm="'cashier.write'" type="primary" :loading="submitting" @click="submit">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { createCashierTemplateVersion, type CreateVersionPayload } from "@/api/modules/cashier";

const props = defineProps<{
  open: boolean;
  templateId: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "created"): void;
}>();

const form = reactive<CreateVersionPayload>({
  sourceType: "manual",
  sourceVersion: ""
});
const submitting = ref(false);

watch(
  () => props.open,
  (open) => {
    if (open) {
      form.sourceType = "manual";
      form.sourceVersion = "";
    }
  }
);

async function submit() {
  if (form.sourceType !== "manual" && !form.sourceVersion) {
    ElMessage.warning("复制创建时请填写来源版本");
    return;
  }

  submitting.value = true;
  try {
    await createCashierTemplateVersion(props.templateId, {
      sourceType: form.sourceType,
      sourceVersion: form.sourceType === "manual" ? undefined : form.sourceVersion || undefined
    });
    ElMessage.success("draft 版本已创建");
    emit("created");
    emit("close");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "创建版本失败");
  } finally {
    submitting.value = false;
  }
}
</script>
