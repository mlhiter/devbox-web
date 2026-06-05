import * as React from 'react';
import { useTranslations } from 'next-intl';
import { TriangleAlert } from 'lucide-react';
import { useCallback, useRef, useState } from 'react';

import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@labring/sealos-ui';
import { Button } from '@labring/sealos-ui/button';

const confirmDialogContentStyle: React.CSSProperties = {
  top: '20%',
  left: '50%',
  translate: '-50% 0',
  animation: 'none',
  transitionDuration: '0ms'
};

export const useConfirm = ({
  title = 'prompt',
  content,
  contentParams,
  confirmText = 'confirm',
  cancelText = 'cancel',
  confirmButtonVariant
}: {
  title?: string;
  content: string;
  contentParams?: Record<string, string | number>;
  confirmText?: string;
  cancelText?: string;
  confirmButtonVariant?: React.ComponentProps<typeof Button>['variant'];
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const t = useTranslations();
  const confirmCb = useRef<any>();
  const cancelCb = useRef<any>();

  return {
    openConfirm: useCallback((confirm?: any, cancel?: any) => {
      return function () {
        setIsOpen(true);
        confirmCb.current = confirm;
        cancelCb.current = cancel;
      };
    }, []),
    ConfirmChild: useCallback(
      () => (
        <Dialog open={isOpen} onOpenChange={setIsOpen}>
          <DialogContent className="w-[400px]" style={confirmDialogContentStyle}>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-1.5">
                <TriangleAlert className="h-4 w-4 text-yellow-600" />
                {t(title)}
              </DialogTitle>
            </DialogHeader>
            <div className="text-zinc-900">{t(content, contentParams)}</div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setIsOpen(false);
                  typeof cancelCb.current === 'function' && cancelCb.current();
                }}
              >
                {t(cancelText)}
              </Button>
              <Button
                variant={confirmButtonVariant}
                onClick={() => {
                  setIsOpen(false);
                  typeof confirmCb.current === 'function' && confirmCb.current();
                }}
              >
                {t(confirmText)}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ),
      [cancelText, confirmButtonVariant, confirmText, content, contentParams, isOpen, t, title]
    )
  };
};
