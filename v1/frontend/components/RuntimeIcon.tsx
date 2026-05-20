'use client';

import Image from 'next/image';
import { useEffect, useState } from 'react';

interface RuntimeIconProps {
  iconId: string | null;
  alt: string;
  width?: number;
  height?: number;
  className?: string;
  priority?: boolean;
  onLoad?: () => void;
}

const fallbackRuntimeIcon = '/images/runtime/custom.svg';

const getRuntimeIconSrc = (iconId: string | null) =>
  iconId ? `/images/runtime/${iconId}.svg` : fallbackRuntimeIcon;

export const RuntimeIcon = ({
  iconId,
  alt,
  width = 21,
  height = 21,
  className,
  priority,
  onLoad
}: RuntimeIconProps) => {
  const [imgSrc, setImgSrc] = useState(getRuntimeIconSrc(iconId));

  useEffect(() => {
    setImgSrc(getRuntimeIconSrc(iconId));
  }, [iconId]);

  return (
    <Image
      width={width}
      height={height}
      alt={alt}
      src={imgSrc}
      className={className}
      priority={priority}
      onError={() => setImgSrc(fallbackRuntimeIcon)}
      onLoad={onLoad}
    />
  );
};
