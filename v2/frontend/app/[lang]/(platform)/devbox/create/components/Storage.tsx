'use client';

import { useTranslations } from 'next-intl';
import { useFormContext } from 'react-hook-form';

import { DevboxEditTypeV2 } from '@/types/devbox';

import { Label } from '@labring/sealos-ui/label';
import { Slider } from '@labring/sealos-ui/slider';

const StorageSlideMarkList = [
  { label: '10', value: 10 },
  { label: '20', value: 20 },
  { label: '30', value: 30 },
  { label: '40', value: 40 },
  { label: '50', value: 50 }
];

export default function Storage() {
  const t = useTranslations();
  const { watch, setValue } = useFormContext<DevboxEditTypeV2>();

  const currentValue = watch('storageLimit') || '10Gi';
  const currentStorage = Number.parseInt(currentValue.replace(/Gi/i, ''), 10);
  const currentIndex = StorageSlideMarkList.findIndex((item) => item.value === currentStorage);

  return (
    <div className="flex items-start gap-10">
      <Label className="w-15 font-medium text-gray-900">{t('storage')}</Label>
      <div className="flex-1">
        <Slider
          value={[currentIndex !== -1 ? currentIndex : 0]}
          onValueChange={(values) => {
            const index = values[0];
            setValue('storageLimit', `${StorageSlideMarkList[index].value}Gi`);
          }}
          max={StorageSlideMarkList.length - 1}
          min={0}
          step={1}
          marks={StorageSlideMarkList}
        />
      </div>
    </div>
  );
}
