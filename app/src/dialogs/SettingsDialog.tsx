import "@/assets/pages/settings.less";
import { useTranslation } from "react-i18next";
import { useDispatch, useSelector } from "react-redux";
import * as settings from "@/store/settings.ts";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog.tsx";
import { Checkbox } from "@/components/ui/checkbox.tsx";
import { useEffect, useState } from "react";
import { getMemoryPerformance } from "@/utils/app.ts";
import { version } from "@/conf/bootstrap.ts";
import { NumberInput } from "@/components/ui/number-input.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select.tsx";
import { langsProps, setLanguage } from "@/i18n.ts";
import { cn } from "@/components/ui/lib/utils.ts";
import Tips from "@/components/Tips.tsx";
import { Button } from "@/components/ui/button.tsx";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog.tsx";
import { Badge } from "@/components/ui/badge.tsx";
import Github from "@/components/ui/icons/Github.tsx";
import { isTauri } from "@/utils/desktop.ts";
import { useDeeptrain } from "@/conf/env.ts";
import ThemeToggle from "@/components/ThemeProvider.tsx";

function SettingsDialog() {
  const { t, i18n } = useTranslation();
  const dispatch = useDispatch();

  const open = useSelector(settings.dialogSelector);

  const align = useSelector(settings.alignSelector);
  const hideToolbar = useSelector(settings.hideToolbarSelector);
  const hideToolbarText = useSelector(settings.hideToolbarTextSelector);
  const context = useSelector(settings.contextSelector);
  const sender = useSelector(settings.senderSelector);
  const history = useSelector(settings.historySelector);

  const maxTokens = useSelector(settings.maxTokensSelector);

  const [memorySize, setMemorySize] = useState(getMemoryPerformance());

  const desktop = isTauri();

  useEffect(() => {
    const interval = setInterval(() => {
      setMemorySize(getMemoryPerformance());
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  return (
    <Dialog
      open={open}
      onOpenChange={(open) => dispatch(settings.setDialog(open))}
    >
      <DialogContent className={`flex-dialog settings-dialog`} couldFullScreen>
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
          <DialogDescription asChild>
            <div className={`settings-container`}>
              <div className={`settings-wrapper`}>
                <div className={`settings-segment`}>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.version")}</div>
                    <div className={`grow`} />
                    <div className={`value`}>
                      v{version}
                      <Badge className={`ml-1`}>Community</Badge>
                    </div>
                  </div>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.theme")}</div>
                    <div className={`grow`} />
                    <div className={`value`}>
                      <ThemeToggle />
                    </div>
                  </div>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.language")}</div>
                    <div className={`grow`} />
                    <div className={`value`}>
                      <Select
                        value={i18n.language}
                        onValueChange={(value: string) =>
                          setLanguage(i18n, value)
                        }
                      >
                        <SelectTrigger className={`select`}>
                          <SelectValue
                            placeholder={langsProps[i18n.language]}
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {Object.entries(langsProps).map(
                            ([key, value], idx) => (
                              <SelectItem key={idx} value={key}>
                                {value}
                              </SelectItem>
                            ),
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
                <div className={`settings-segment`}>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.sender")}</div>
                    <div className={`grow`} />
                    <div className={`value`}>
                      <Select
                        value={sender ? "true" : "false"}
                        onValueChange={(value: string) =>
                          dispatch(settings.setSender(value === "true"))
                        }
                      >
                        <SelectTrigger className={`select`}>
                          <SelectValue
                            placeholder={settings.sendKeys[sender ? 1 : 0]}
                          />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value={"false"}>
                            {settings.sendKeys[0]}
                          </SelectItem>
                          <SelectItem value={"true"}>
                            {settings.sendKeys[1]}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.align")}</div>
                    <div className={`grow`} />
                    <Checkbox
                      className={`value`}
                      checked={align}
                      onCheckedChange={(state: boolean) => {
                        dispatch(settings.setAlign(state));
                      }}
                    />
                  </div>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.hide-toolbar")}</div>
                    <div className={`grow`} />
                    <Checkbox
                      className={`value`}
                      checked={hideToolbar}
                      onCheckedChange={(state: boolean) => {
                        dispatch(settings.setHideToolbar(state));
                      }}
                    />
                  </div>
                  <div className={`item`}>
                    <div className={`name`}>
                      {t("settings.hide-toolbar-text")}
                    </div>
                    <div className={`grow`} />
                    <Checkbox
                      className={`value`}
                      checked={hideToolbarText}
                      onCheckedChange={(state: boolean) => {
                        dispatch(settings.setHideToolbarText(state));
                      }}
                    />
                  </div>
                </div>
                <div className={`settings-segment`}>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.context")}</div>
                    <div className={`grow`} />
                    <Checkbox
                      className={`value`}
                      checked={context}
                      onCheckedChange={(state: boolean) => {
                        dispatch(settings.setContext(state));
                      }}
                    />
                  </div>
                  {context && (
                    <div className={`item`}>
                      <div className={`name`}>{t("settings.history")}</div>
                      <div className={`grow`} />
                      <NumberInput
                        className={cn(
                          `value`,
                          history === 0 && `text-destructive`,
                        )}
                        value={history}
                        acceptNaN={false}
                        min={0}
                        max={999}
                        onValueChange={(value: number) => {
                          dispatch(settings.setHistory(value));
                        }}
                      />
                    </div>
                  )}
                  <div className={`item`}>
                    <div className={`name`}>
                      {t("settings.max-tokens")}
                      <Tips content={t("settings.max-tokens-tip")} />
                    </div>
                    <div className={`grow`} />
                    <NumberInput
                      className={`value large-value`}
                      value={maxTokens}
                      acceptNaN={false}
                      min={1}
                      max={100000}
                      onValueChange={(value: number) => {
                        dispatch(settings.setMaxTokens(value));
                      }}
                    />
                  </div>
                </div>
                <div className={`settings-segment`}>
                  <div className={`item`}>
                    <div className={`name`}>{t("settings.reset-settings")}</div>
                    <div className={`grow`} />
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          size={`sm`}
                          variant={`destructive`}
                          className={`set-action`}
                        >
                          {t("reset")}
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>
                            {t("settings.reset-settings")}
                          </AlertDialogTitle>
                          <AlertDialogDescription>
                            {t("settings.reset-settings-description")}
                          </AlertDialogDescription>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => {
                                dispatch(settings.resetSettings());
                              }}
                            >
                              {t("confirm")}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogHeader>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </div>
              </div>
              <div className={`grow`} />
              <div className={`info-box`}>
                <p>
                  {t("settings.memory")}
                  &nbsp;
                  {!isNaN(memorySize)
                    ? memorySize.toFixed(2) + " MB"
                    : t("unknown")}
                </p>
                <a
                  className={cn(
                    "flex flex-row items-center",
                    !useDeeptrain && "hidden",
                  )}
                  href={`https://github.com/coaidev/coai`}
                >
                  <Github className={`inline-block h-4 w-4 mr-1.5`} />
                  CoAI v{version}
                  {desktop && <Badge className={`ml-1`}>App</Badge>}
                </a>
              </div>
            </div>
          </DialogDescription>
        </DialogHeader>
      </DialogContent>
    </Dialog>
  );
}

export default SettingsDialog;
