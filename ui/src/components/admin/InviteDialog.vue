<script setup>
import { Check, Copy } from "lucide-vue-next";
import { ref, watch } from "vue";

import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import FormField from "@/components/ui/FormField.vue";
import SwitchRow from "@/components/ui/SwitchRow.vue";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { useToast } from "@/stores/toast";

/**
 * Creates accounts for one or more addresses in a group.
 *
 * New accounts have no password: each owner chooses one through a link,
 * which is e-mailed to them when asked and shown here either way, for the
 * administrator to pass on. An address that already has an account is
 * moved into the group instead.
 */
const props = defineProps({
  groupId: { type: String, default: "" },
});

const emit = defineEmits(["invited"]);
const open = defineModel("open", { type: Boolean, default: false });

const toast = useToast();

const groups = ref([]);
const emails = ref("");
const group = ref("");
const notify = ref(true);
const errors = ref({});
const busy = ref(false);
const invited = ref(null);
const copied = ref("");

watch(open, async (value) => {
  if (!value) return;

  emails.value = "";
  group.value = props.groupId;
  notify.value = true;
  errors.value = {};
  invited.value = null;

  const { data } = await api.get("/admin/groups/all");
  groups.value = data;
});

async function invite() {
  busy.value = true;
  errors.value = {};

  try {
    const { data } = await api.post("/admin/users", {
      // The server splits on whitespace only, so a pasted comma separated
      // list would otherwise lose every address with a comma stuck to it.
      email: emails.value.replace(/[,;]/g, " "),
      group_id: group.value,
      notify: notify.value,
    });
    emit("invited");

    if (data.length) {
      invited.value = data;
    } else {
      open.value = false;
      toast.success("Existing users moved into the group.");
    }
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!errors.value.email && !errors.value.group_id) toast.error(errorMessage(error));
  } finally {
    busy.value = false;
  }
}

async function copy(text, key) {
  await navigator.clipboard.writeText(text);
  copied.value = key;
  setTimeout(() => (copied.value = ""), 1500);
}

function allLinks() {
  return invited.value.map((user) => `${user.email}\t${user.link}`).join("\n");
}
</script>

<template>
  <AppDialog
    v-model:open="open"
    :title="invited ? 'Users invited' : 'Invite users'"
    :description="
      invited
        ? (invited.every((user) => user.emailed)
            ? 'We\'ve emailed each of them a link to choose a password. '
            : notify
              ? 'This server can\'t send email yet, so pass each of them their link to choose a password. '
              : 'Pass each of them their link to choose a password. ') + 'Links work once, for 7 days.'
        : 'Paste one or more email addresses. Anyone who already has an account is moved into the group.'
    "
    :busy="busy"
    width="max-w-[560px]"
  >
    <template v-if="!invited">
      <form id="invite-users" class="flex flex-col gap-5" @submit.prevent="invite">
        <FormField
          v-slot="field"
          label="Email addresses"
          hint="Separate addresses with spaces, commas or new lines."
          :error="errors.email"
        >
          <textarea
            :id="field.id"
            v-model="emails"
            rows="4"
            autofocus
            placeholder="ada@example.com&#10;grace@example.com"
            class="input font-mono text-[13px]"
            :aria-invalid="field.invalid"
            :aria-describedby="field.describedby"
          />
        </FormField>

        <FormField v-slot="field" label="Group" :error="errors.group_id">
          <select
            :id="field.id"
            v-model="group"
            class="input"
            :aria-invalid="field.invalid"
            :aria-describedby="field.describedby"
          >
            <option value="" disabled>Choose a group</option>
            <option v-for="option in groups" :key="option.id" :value="option.id">{{ option.name }}</option>
          </select>
        </FormField>

        <SwitchRow
          v-model="notify"
          label="Email invitations"
          description="Each new user gets a link to choose their password."
        />
      </form>
    </template>

    <div v-else class="overflow-hidden rounded-[10px] border border-line">
      <div
        v-for="user in invited"
        :key="user.email"
        class="flex items-center gap-3 border-b border-hair px-4 py-3 last:border-0"
      >
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span class="truncate font-medium">{{ user.email }}</span>
          <span class="truncate font-mono text-[12px] text-ink-3">{{ user.link }}</span>
        </div>
        <button
          type="button"
          :aria-label="`Copy the link for ${user.email}`"
          class="flex size-8 shrink-0 items-center justify-center rounded-lg text-ink-4 hover:bg-muted hover:text-ink"
          @click="copy(user.link, user.email)"
        >
          <Check v-if="copied === user.email" class="size-4 text-up" :stroke-width="2" />
          <Copy v-else class="size-4" :stroke-width="2" />
        </button>
      </div>
    </div>

    <template #footer>
      <template v-if="!invited">
        <AppButton :disabled="busy" @click="open = false">Cancel</AppButton>
        <AppButton variant="primary" type="submit" form="invite-users" :disabled="busy">
          {{ busy ? "Inviting…" : "Invite" }}
        </AppButton>
      </template>
      <template v-else>
        <AppButton @click="copy(allLinks(), '*')">
          <Check v-if="copied === '*'" class="size-4 text-up" :stroke-width="2" />
          <Copy v-else class="size-4" :stroke-width="2" />
          Copy all
        </AppButton>
        <AppButton variant="primary" @click="open = false">Done</AppButton>
      </template>
    </template>
  </AppDialog>
</template>
