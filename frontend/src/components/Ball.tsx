// File: frontend/src/components/Ball.tsx
import styled from 'styled-components';
import { Ball as BallType } from '../types/game';

// Google Colors
const BALL_COLOR = '#4285F4'; // Blue

interface BallProps {
  $ballData: BallType;
}

// Position using top/left based on center coordinates minus radius
const BallComponent = styled.div<BallProps>`
  position: absolute;
  background-color: ${BALL_COLOR};
  width: ${(props) => props.$ballData.radius * 2}px;
  height: ${(props) => props.$ballData.radius * 2}px;
  left: ${(props) => props.$ballData.x - props.$ballData.radius}px;
  top: ${(props) => props.$ballData.y - props.$ballData.radius}px;
  border-radius: 50%;
  box-shadow: 0 0 8px ${BALL_COLOR};
;
`
export default BallComponent;
